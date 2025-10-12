package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"
)

type MiddlewareFunc func(http.Handler) http.Handler

func chainMiddleware(h http.Handler, middlewares ...MiddlewareFunc) http.Handler {
	for _, mw := range middlewares {
		h = mw(h)
	}
	return h
}

func newAuthMiddleware(tokens []string) MiddlewareFunc {
	tokenSet := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		tokenSet[token] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(tokens) != 0 {
				token := r.Header.Get("Authorization")
				token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
				if token == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				if _, ok := tokenSet[token]; !ok {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func loggerMiddleware(prefix string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("<%s> Request [%s] %s", prefix, r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}

func recoverMiddleware(prefix string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("<%s> Recovered from panic: %v", prefix, err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func startHTTPServer(config *Config) error {
	// Check for recursive lazy load mode
	if config.McpProxy.Options != nil && config.McpProxy.Options.RecursiveLazyLoad.OrElse(false) {
		return startRecursiveProxyServer(config)
	}

	baseURL, uErr := url.Parse(config.McpProxy.BaseURL)
	if uErr != nil {
		return uErr
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var errorGroup errgroup.Group
	httpMux := http.NewServeMux()
	httpServer := &http.Server{
		Addr:    config.McpProxy.Addr,
		Handler: httpMux,
	}
	info := mcp.Implementation{
		Name: config.McpProxy.Name,
	}

	for name, clientConfig := range config.McpServers {
		mcpClient, err := newMCPClient(name, clientConfig)
		if err != nil {
			return err
		}
		server, err := newMCPServer(name, config.McpProxy, clientConfig)
		if err != nil {
			return err
		}
		errorGroup.Go(func() error {
			log.Printf("<%s> Connecting", name)
			addErr := mcpClient.addToMCPServer(ctx, info, server.mcpServer)
			if addErr != nil {
				log.Printf("<%s> Failed to add client to server: %v", name, addErr)
				if clientConfig.Options.PanicIfInvalid.OrElse(false) {
					return addErr
				}
				return nil
			}
			log.Printf("<%s> Connected", name)

			middlewares := make([]MiddlewareFunc, 0)
			middlewares = append(middlewares, recoverMiddleware(name))
			if clientConfig.Options.LogEnabled.OrElse(false) {
				middlewares = append(middlewares, loggerMiddleware(name))
			}
			if len(clientConfig.Options.AuthTokens) > 0 {
				middlewares = append(middlewares, newAuthMiddleware(clientConfig.Options.AuthTokens))
			}
			mcpRoute := path.Join(baseURL.Path, name)
			if !strings.HasPrefix(mcpRoute, "/") {
				mcpRoute = "/" + mcpRoute
			}
			if !strings.HasSuffix(mcpRoute, "/") {
				mcpRoute += "/"
			}
			log.Printf("<%s> Handling requests at %s", name, mcpRoute)
			httpMux.Handle(mcpRoute, chainMiddleware(server.handler, middlewares...))
			httpServer.RegisterOnShutdown(func() {
				log.Printf("<%s> Shutting down", name)
				_ = mcpClient.Close()
			})
			return nil
		})
	}

	go func() {
		err := errorGroup.Wait()
		if err != nil {
			log.Fatalf("Failed to add clients: %v", err)
		}
		log.Printf("All clients initialized")
	}()

	go func() {
		log.Printf("Starting %s server", config.McpProxy.Type)
		log.Printf("%s server listening on %s", config.McpProxy.Type, config.McpProxy.Addr)
		hErr := httpServer.ListenAndServe()
		if hErr != nil && !errors.Is(hErr, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", hErr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	err := httpServer.Shutdown(shutdownCtx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func startRecursiveProxyServer(config *Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Determine hierarchy path - default to testdata/mcp_hierarchy
	hierarchyPath := "testdata/mcp_hierarchy"
	if config.McpProxy.BaseURL != "" {
		// Could potentially support custom hierarchy path via BaseURL or new config field
		// For now, use default
	}

	// Load hierarchy from filesystem
	log.Printf("Loading hierarchy from %s", hierarchyPath)
	hierarchy, err := LoadHierarchy(hierarchyPath)
	if err != nil {
		return fmt.Errorf("failed to load hierarchy: %w", err)
	}

	// Create server registry for lazy-loaded MCP clients
	registry := NewServerRegistry()
	defer registry.Close()

	// Create ONE MCP server with 2 meta-tools
	serverOpts := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	}

	if config.McpProxy.Options != nil && config.McpProxy.Options.LogEnabled.OrElse(false) {
		serverOpts = append(serverOpts, server.WithLogging())
	}

	mcpServer := server.NewMCPServer(
		config.McpProxy.Name,
		config.McpProxy.Version,
		serverOpts...,
	)

	// Register get_tools_in_category meta-tool
	getToolsInCategoryTool := mcp.Tool{
		Name:        "get_tools_in_category",
		Description: "Navigate the tool hierarchy and discover available tools in a category. Returns categories, subcategories, and tools at the specified path.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Category path using dot notation (e.g., 'coding_tools' or 'coding_tools.serena.search'). Use empty string or '/' for root.",
				},
			},
			Required: []string{"path"},
		},
	}

	mcpServer.AddTool(getToolsInCategoryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := ""
		if request.Params.Arguments != nil {
			if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
				if pathVal, ok := argsMap["path"].(string); ok {
					path = pathVal
				}
			}
		}

		response, err := hierarchy.HandleGetToolsInCategory(path)
		if err != nil {
			return nil, err
		}

		jsonBytes, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return nil, err
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(jsonBytes)),
			},
		}, nil
	})

	// Register execute_tool meta-tool
	executeToolTool := mcp.Tool{
		Name:        "execute_tool",
		Description: "Execute a tool by its full path. Automatically proxies the request to the appropriate MCP server.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tool_path": map[string]interface{}{
					"type":        "string",
					"description": "Full tool path using dot notation (e.g., 'coding_tools.serena.search.search_symbol') or just tool name if unique",
				},
				"arguments": map[string]interface{}{
					"type":                 "object",
					"description":          "Arguments to pass to the tool",
					"additionalProperties": true,
				},
			},
			Required: []string{"tool_path", "arguments"},
		},
	}

	mcpServer.AddTool(executeToolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toolPath := ""
		arguments := make(map[string]interface{})

		if request.Params.Arguments != nil {
			if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
				if pathVal, ok := argsMap["tool_path"].(string); ok {
					toolPath = pathVal
				}
				if argsVal, ok := argsMap["arguments"].(map[string]interface{}); ok {
					arguments = argsVal
				}
			}
		}

		if toolPath == "" {
			return nil, fmt.Errorf("tool_path is required")
		}

		return hierarchy.HandleExecuteTool(ctx, registry, toolPath, arguments)
	})

	// Set up HTTP handler (SSE or Streamable)
	var handler http.Handler
	switch config.McpProxy.Type {
	case MCPServerTypeSSE:
		handler = server.NewSSEServer(
			mcpServer,
			server.WithStaticBasePath(""),
			server.WithBaseURL(config.McpProxy.BaseURL),
		)
	case MCPServerTypeStreamable:
		handler = server.NewStreamableHTTPServer(
			mcpServer,
			server.WithStateLess(true),
		)
	default:
		return fmt.Errorf("unknown server type: %s", config.McpProxy.Type)
	}

	// Apply middleware
	middlewares := make([]MiddlewareFunc, 0)
	middlewares = append(middlewares, recoverMiddleware("recursive-proxy"))
	if config.McpProxy.Options != nil && config.McpProxy.Options.LogEnabled.OrElse(false) {
		middlewares = append(middlewares, loggerMiddleware("recursive-proxy"))
	}
	if config.McpProxy.Options != nil && len(config.McpProxy.Options.AuthTokens) > 0 {
		middlewares = append(middlewares, newAuthMiddleware(config.McpProxy.Options.AuthTokens))
	}
	handler = chainMiddleware(handler, middlewares...)

	// Start HTTP server
	httpMux := http.NewServeMux()
	httpMux.Handle("/", handler)

	httpServer := &http.Server{
		Addr:    config.McpProxy.Addr,
		Handler: httpMux,
	}

	go func() {
		log.Printf("Starting recursive lazy loading %s server", config.McpProxy.Type)
		log.Printf("%s server listening on %s", config.McpProxy.Type, config.McpProxy.Addr)
		hErr := httpServer.ListenAndServe()
		if hErr != nil && !errors.Is(hErr, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", hErr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	err = httpServer.Shutdown(shutdownCtx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
