package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Client struct {
	name            string
	needPing        bool
	needManualStart bool
	client          *client.Client
	options         *OptionsV2
	// Lazy loading fields
	mcpServer     *server.MCPServer
	lazyTools     []mcp.Tool
	lazyPrompts   []mcp.Prompt
	lazyResources []mcp.Resource
	lazyTemplates []mcp.ResourceTemplate
	activateOnce  sync.Once
	activated     bool
}

func newMCPClient(name string, conf *MCPClientConfigV2) (*Client, error) {
	clientInfo, pErr := parseMCPClientConfigV2(conf)
	if pErr != nil {
		return nil, pErr
	}
	switch v := clientInfo.(type) {
	case *StdioMCPClientConfig:
		envs := make([]string, 0, len(v.Env))
		for kk, vv := range v.Env {
			envs = append(envs, fmt.Sprintf("%s=%s", kk, vv))
		}
		mcpClient, err := client.NewStdioMCPClient(v.Command, envs, v.Args...)
		if err != nil {
			return nil, err
		}

		return &Client{
			name:    name,
			client:  mcpClient,
			options: conf.Options,
		}, nil
	case *SSEMCPClientConfig:
		var options []transport.ClientOption
		if len(v.Headers) > 0 {
			options = append(options, client.WithHeaders(v.Headers))
		}
		mcpClient, err := client.NewSSEMCPClient(v.URL, options...)
		if err != nil {
			return nil, err
		}
		return &Client{
			name:            name,
			needPing:        true,
			needManualStart: true,
			client:          mcpClient,
			options:         conf.Options,
		}, nil
	case *StreamableMCPClientConfig:
		var options []transport.StreamableHTTPCOption
		if len(v.Headers) > 0 {
			options = append(options, transport.WithHTTPHeaders(v.Headers))
		}
		if v.Timeout > 0 {
			options = append(options, transport.WithHTTPTimeout(v.Timeout))
		}
		mcpClient, err := client.NewStreamableHttpClient(v.URL, options...)
		if err != nil {
			return nil, err
		}
		return &Client{
			name:            name,
			needPing:        true,
			needManualStart: true,
			client:          mcpClient,
			options:         conf.Options,
		}, nil
	}
	return nil, errors.New("invalid client type")
}

func (c *Client) addToMCPServer(ctx context.Context, clientInfo mcp.Implementation, mcpServer *server.MCPServer) error {
	// Store mcpServer reference for later activation
	c.mcpServer = mcpServer

	if c.needManualStart {
		err := c.client.Start(ctx)
		if err != nil {
			return err
		}
	}
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = clientInfo
	initRequest.Params.Capabilities = mcp.ClientCapabilities{
		Experimental: make(map[string]interface{}),
		Roots:        nil,
		Sampling:     nil,
	}
	_, err := c.client.Initialize(ctx, initRequest)
	if err != nil {
		return err
	}
	log.Printf("<%s> Successfully initialized MCP client", c.name)

	// Check if lazy loading is enabled
	if c.options != nil && c.options.LazyLoad.OrElse(false) {
		// Lazy loading mode: store tools/prompts/resources without registering them
		err = c.storeToolsForLazyLoad(ctx)
		if err != nil {
			return err
		}
		_ = c.storePromptsForLazyLoad(ctx)
		_ = c.storeResourcesForLazyLoad(ctx)
		_ = c.storeResourceTemplatesForLazyLoad(ctx)

		// Register the meta-tool for activation
		c.registerMetaTool()
	} else {
		// Normal mode: register everything immediately
		err = c.addToolsToServer(ctx, mcpServer)
		if err != nil {
			return err
		}
		_ = c.addPromptsToServer(ctx, mcpServer)
		_ = c.addResourcesToServer(ctx, mcpServer)
		_ = c.addResourceTemplatesToServer(ctx, mcpServer)
	}

	if c.needPing {
		go c.startPingTask(ctx)
	}
	return nil
}

// activateTools is called when the meta-tool is invoked to load all real tools
func (c *Client) activateTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var activationErr error
	var toolCount, promptCount, resourceCount, templateCount int

	c.activateOnce.Do(func() {
		log.Printf("<%s> Activating lazy-loaded tools, prompts, and resources", c.name)

		// Register all stored tools
		toolCount = 0
		for _, tool := range c.lazyTools {
			log.Printf("<%s> Adding tool %s", c.name, tool.Name)
			c.mcpServer.AddTool(tool, c.client.CallTool)
			toolCount++
		}

		// Register all stored prompts
		promptCount = 0
		for _, prompt := range c.lazyPrompts {
			log.Printf("<%s> Adding prompt %s", c.name, prompt.Name)
			c.mcpServer.AddPrompt(prompt, c.client.GetPrompt)
			promptCount++
		}

		// Register all stored resources
		resourceCount = 0
		for _, resource := range c.lazyResources {
			log.Printf("<%s> Adding resource %s", c.name, resource.Name)
			c.mcpServer.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				readResource, e := c.client.ReadResource(ctx, request)
				if e != nil {
					return nil, e
				}
				return readResource.Contents, nil
			})
			resourceCount++
		}

		// Register all stored resource templates
		templateCount = 0
		for _, template := range c.lazyTemplates {
			log.Printf("<%s> Adding resource template %s", c.name, template.Name)
			c.mcpServer.AddResourceTemplate(template, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				readResource, e := c.client.ReadResource(ctx, request)
				if e != nil {
					return nil, e
				}
				return readResource.Contents, nil
			})
			templateCount++
		}

		// Clear the lazy storage to prevent double registration
		c.lazyTools = nil
		c.lazyPrompts = nil
		c.lazyResources = nil
		c.lazyTemplates = nil
		c.activated = true

		log.Printf("<%s> Activation complete: %d tools, %d prompts, %d resources, %d templates",
			c.name, toolCount, promptCount, resourceCount, templateCount)
	})

	if activationErr != nil {
		return nil, activationErr
	}

	// Return success response
	response := map[string]interface{}{
		"activated":      true,
		"server":         c.name,
		"toolCount":      toolCount,
		"promptCount":    promptCount,
		"resourceCount":  resourceCount,
		"templateCount":  templateCount,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(jsonBytes)),
		},
	}, nil
}

// registerMetaTool creates and registers the activation meta-tool
func (c *Client) registerMetaTool() {
	metaToolName := fmt.Sprintf("activate_%s", c.name)

	// Build description with server-specific context
	var description string
	switch c.name {
	case "serena":
		description = "Activate Serena MCP server. Provides semantic code operations, symbol finding, file editing, and code analysis tools. "
	case "playwright":
		description = "Activate Playwright MCP server. Provides browser automation, web scraping, screenshots, and web interaction tools. "
	default:
		description = fmt.Sprintf("Activate and load all tools from the %s MCP server. ", c.name)
	}

	// Add counts of what will be loaded
	description += fmt.Sprintf("This will load %d tools", len(c.lazyTools))
	if len(c.lazyPrompts) > 0 {
		description += fmt.Sprintf(", %d prompts", len(c.lazyPrompts))
	}
	if len(c.lazyResources) > 0 {
		description += fmt.Sprintf(", %d resources", len(c.lazyResources))
	}
	if len(c.lazyTemplates) > 0 {
		description += fmt.Sprintf(", %d resource templates", len(c.lazyTemplates))
	}
	description += "."

	// Add hints about what this server provides based on tool names
	if len(c.lazyTools) > 0 {
		description += " Available tools include: "
		toolNames := make([]string, 0, 5)
		for i, tool := range c.lazyTools {
			if i < 5 {
				toolNames = append(toolNames, tool.Name)
			} else {
				break
			}
		}
		description += strings.Join(toolNames, ", ")
		if len(c.lazyTools) > 5 {
			description += fmt.Sprintf(" and %d more", len(c.lazyTools)-5)
		}
		description += "."
	}

	metaTool := mcp.Tool{
		Name:        metaToolName,
		Description: description,
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}

	log.Printf("<%s> Registering meta-tool: %s", c.name, metaToolName)
	c.mcpServer.AddTool(metaTool, c.activateTools)
}

func (c *Client) startPingTask(ctx context.Context) {
	interval := 30 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("<%s> Context done, stopping ping", c.name)
			return
		case <-ticker.C:
			if err := c.client.Ping(ctx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				failCount++
				log.Printf("<%s> MCP Ping failed: %v (count=%d)", c.name, err, failCount)
			} else if failCount > 0 {
				log.Printf("<%s> MCP Ping recovered after %d failures", c.name, failCount)
				failCount = 0
			}
		}
	}
}

func (c *Client) addToolsToServer(ctx context.Context, mcpServer *server.MCPServer) error {
	toolsRequest := mcp.ListToolsRequest{}
	filterFunc := func(toolName string) bool {
		return true
	}

	if c.options != nil && c.options.ToolFilter != nil && len(c.options.ToolFilter.List) > 0 {
		filterSet := make(map[string]struct{})
		mode := ToolFilterMode(strings.ToLower(string(c.options.ToolFilter.Mode)))
		for _, toolName := range c.options.ToolFilter.List {
			filterSet[toolName] = struct{}{}
		}
		switch mode {
		case ToolFilterModeAllow:
			filterFunc = func(toolName string) bool {
				_, inList := filterSet[toolName]
				if !inList {
					log.Printf("<%s> Ignoring tool %s as it is not in allow list", c.name, toolName)
				}
				return inList
			}
		case ToolFilterModeBlock:
			filterFunc = func(toolName string) bool {
				_, inList := filterSet[toolName]
				if inList {
					log.Printf("<%s> Ignoring tool %s as it is in block list", c.name, toolName)
				}
				return !inList
			}
		default:
			log.Printf("<%s> Unknown tool filter mode: %s, skipping tool filter", c.name, mode)
		}
	}

	for {
		tools, err := c.client.ListTools(ctx, toolsRequest)
		if err != nil {
			return err
		}
		if len(tools.Tools) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d tools", c.name, len(tools.Tools))
		for _, tool := range tools.Tools {
			if filterFunc(tool.Name) {
				log.Printf("<%s> Adding tool %s", c.name, tool.Name)
				mcpServer.AddTool(tool, c.client.CallTool)
			}
		}
		if tools.NextCursor == "" {
			break
		}
		toolsRequest.Params.Cursor = tools.NextCursor
	}

	return nil
}

func (c *Client) addPromptsToServer(ctx context.Context, mcpServer *server.MCPServer) error {
	promptsRequest := mcp.ListPromptsRequest{}
	for {
		prompts, err := c.client.ListPrompts(ctx, promptsRequest)
		if err != nil {
			return err
		}
		if len(prompts.Prompts) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d prompts", c.name, len(prompts.Prompts))
		for _, prompt := range prompts.Prompts {
			log.Printf("<%s> Adding prompt %s", c.name, prompt.Name)
			mcpServer.AddPrompt(prompt, c.client.GetPrompt)
		}
		if prompts.NextCursor == "" {
			break
		}
		promptsRequest.Params.Cursor = prompts.NextCursor
	}
	return nil
}

func (c *Client) addResourcesToServer(ctx context.Context, mcpServer *server.MCPServer) error {
	resourcesRequest := mcp.ListResourcesRequest{}
	for {
		resources, err := c.client.ListResources(ctx, resourcesRequest)
		if err != nil {
			return err
		}
		if len(resources.Resources) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d resources", c.name, len(resources.Resources))
		for _, resource := range resources.Resources {
			log.Printf("<%s> Adding resource %s", c.name, resource.Name)
			mcpServer.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				readResource, e := c.client.ReadResource(ctx, request)
				if e != nil {
					return nil, e
				}
				return readResource.Contents, nil
			})
		}
		if resources.NextCursor == "" {
			break
		}
		resourcesRequest.Params.Cursor = resources.NextCursor

	}
	return nil
}

func (c *Client) addResourceTemplatesToServer(ctx context.Context, mcpServer *server.MCPServer) error {
	resourceTemplatesRequest := mcp.ListResourceTemplatesRequest{}
	for {
		resourceTemplates, err := c.client.ListResourceTemplates(ctx, resourceTemplatesRequest)
		if err != nil {
			return err
		}
		if len(resourceTemplates.ResourceTemplates) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d resource templates", c.name, len(resourceTemplates.ResourceTemplates))
		for _, resourceTemplate := range resourceTemplates.ResourceTemplates {
			log.Printf("<%s> Adding resource template %s", c.name, resourceTemplate.Name)
			mcpServer.AddResourceTemplate(resourceTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				readResource, e := c.client.ReadResource(ctx, request)
				if e != nil {
					return nil, e
				}
				return readResource.Contents, nil
			})
		}
		if resourceTemplates.NextCursor == "" {
			break
		}
		resourceTemplatesRequest.Params.Cursor = resourceTemplates.NextCursor
	}
	return nil
}

// storeToolsForLazyLoad fetches and stores tools without registering them
func (c *Client) storeToolsForLazyLoad(ctx context.Context) error {
	toolsRequest := mcp.ListToolsRequest{}
	filterFunc := func(toolName string) bool {
		return true
	}

	if c.options != nil && c.options.ToolFilter != nil && len(c.options.ToolFilter.List) > 0 {
		filterSet := make(map[string]struct{})
		mode := ToolFilterMode(strings.ToLower(string(c.options.ToolFilter.Mode)))
		for _, toolName := range c.options.ToolFilter.List {
			filterSet[toolName] = struct{}{}
		}
		switch mode {
		case ToolFilterModeAllow:
			filterFunc = func(toolName string) bool {
				_, inList := filterSet[toolName]
				if !inList {
					log.Printf("<%s> Ignoring tool %s as it is not in allow list", c.name, toolName)
				}
				return inList
			}
		case ToolFilterModeBlock:
			filterFunc = func(toolName string) bool {
				_, inList := filterSet[toolName]
				if inList {
					log.Printf("<%s> Ignoring tool %s as it is in block list", c.name, toolName)
				}
				return !inList
			}
		default:
			log.Printf("<%s> Unknown tool filter mode: %s, skipping tool filter", c.name, mode)
		}
	}

	for {
		tools, err := c.client.ListTools(ctx, toolsRequest)
		if err != nil {
			return err
		}
		if len(tools.Tools) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d tools for lazy loading", c.name, len(tools.Tools))
		for _, tool := range tools.Tools {
			if filterFunc(tool.Name) {
				c.lazyTools = append(c.lazyTools, tool)
			}
		}
		if tools.NextCursor == "" {
			break
		}
		toolsRequest.Params.Cursor = tools.NextCursor
	}

	return nil
}

// storePromptsForLazyLoad fetches and stores prompts without registering them
func (c *Client) storePromptsForLazyLoad(ctx context.Context) error {
	promptsRequest := mcp.ListPromptsRequest{}
	for {
		prompts, err := c.client.ListPrompts(ctx, promptsRequest)
		if err != nil {
			return err
		}
		if len(prompts.Prompts) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d prompts for lazy loading", c.name, len(prompts.Prompts))
		for _, prompt := range prompts.Prompts {
			c.lazyPrompts = append(c.lazyPrompts, prompt)
		}
		if prompts.NextCursor == "" {
			break
		}
		promptsRequest.Params.Cursor = prompts.NextCursor
	}
	return nil
}

// storeResourcesForLazyLoad fetches and stores resources without registering them
func (c *Client) storeResourcesForLazyLoad(ctx context.Context) error {
	resourcesRequest := mcp.ListResourcesRequest{}
	for {
		resources, err := c.client.ListResources(ctx, resourcesRequest)
		if err != nil {
			return err
		}
		if len(resources.Resources) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d resources for lazy loading", c.name, len(resources.Resources))
		for _, resource := range resources.Resources {
			c.lazyResources = append(c.lazyResources, resource)
		}
		if resources.NextCursor == "" {
			break
		}
		resourcesRequest.Params.Cursor = resources.NextCursor

	}
	return nil
}

// storeResourceTemplatesForLazyLoad fetches and stores resource templates without registering them
func (c *Client) storeResourceTemplatesForLazyLoad(ctx context.Context) error {
	resourceTemplatesRequest := mcp.ListResourceTemplatesRequest{}
	for {
		resourceTemplates, err := c.client.ListResourceTemplates(ctx, resourceTemplatesRequest)
		if err != nil {
			return err
		}
		if len(resourceTemplates.ResourceTemplates) == 0 {
			break
		}
		log.Printf("<%s> Successfully listed %d resource templates for lazy loading", c.name, len(resourceTemplates.ResourceTemplates))
		for _, resourceTemplate := range resourceTemplates.ResourceTemplates {
			c.lazyTemplates = append(c.lazyTemplates, resourceTemplate)
		}
		if resourceTemplates.NextCursor == "" {
			break
		}
		resourceTemplatesRequest.Params.Cursor = resourceTemplates.NextCursor
	}
	return nil
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

type Server struct {
	tokens    []string
	mcpServer *server.MCPServer
	handler   http.Handler
}

func newMCPServer(name string, serverConfig *MCPProxyConfigV2, clientConfig *MCPClientConfigV2) (*Server, error) {
	serverOpts := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	}

	if clientConfig.Options.LogEnabled.OrElse(false) {
		serverOpts = append(serverOpts, server.WithLogging())
	}
	mcpServer := server.NewMCPServer(
		name,
		serverConfig.Version,
		serverOpts...,
	)

	var handler http.Handler

	switch serverConfig.Type {
	case MCPServerTypeSSE:
		handler = server.NewSSEServer(
			mcpServer,
			server.WithStaticBasePath(name),
			server.WithBaseURL(serverConfig.BaseURL),
		)
	case MCPServerTypeStreamable:
		handler = server.NewStreamableHTTPServer(
			mcpServer,
			server.WithStateLess(true),
		)
	default:
		return nil, fmt.Errorf("unknown server type: %s", serverConfig.Type)
	}
	srv := &Server{
		mcpServer: mcpServer,
		handler:   handler,
	}

	if clientConfig.Options != nil && len(clientConfig.Options.AuthTokens) > 0 {
		srv.tokens = clientConfig.Options.AuthTokens
	}

	return srv, nil
}
