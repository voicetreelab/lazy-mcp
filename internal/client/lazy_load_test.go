package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/voicetreelab/lazy-mcp/internal/config"
	"github.com/TBXark/optional-go"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLazyLoadingFlow tests the complete lazy-loading workflow:
// 1. Server starts and exposes only meta-tools (one per MCP server)
// 2. Meta-tools contain summary descriptions of what each server provides
// 3. Calling the activate tool loads all real tools/prompts from that server
func TestLazyLoadingFlow(t *testing.T) {
	// Skip if the required servers aren't available
	// This can be run manually or in CI with the servers set up
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check for SERENA_PATH environment variable
	serenaPath := os.Getenv("SERENA_PATH")
	if serenaPath == "" {
		t.Skip("SERENA_PATH environment variable not set - set it to run this test (e.g., export SERENA_PATH=/path/to/serena)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create test config with Serena and Playwright servers
	cfg := &config.Config{
		McpProxy: &config.MCPProxyConfigV2{
			BaseURL: "http://localhost",
			Addr:    ":0", // Use random port
			Name:    "Test MCP Proxy",
			Version: "1.0.0",
			Type:    config.MCPServerTypeStreamable,
			Options: &config.OptionsV2{
				LazyLoad: optional.NewField(true),
			},
		},
		McpServers: map[string]*config.MCPClientConfigV2{
			"serena": {
				TransportType: config.MCPClientTypeStdio,
				Command:       "uv",
				Args: []string{
					"--directory",
					serenaPath,
					"run",
					"serena",
					"start-mcp-server",
					"--context",
					"claude-code",
				},
				Env: map[string]string{},
				Options: &config.OptionsV2{
					LazyLoad: optional.NewField(true),
				},
			},
		},
	}

	// Start the HTTP server
	httpMux := http.NewServeMux()
	testServer := httptest.NewServer(httpMux)
	defer testServer.Close()

	info := mcp.Implementation{
		Name: cfg.McpProxy.Name,
	}

	// Initialize the Serena client
	mcpClient, err := NewMCPClient("serena", cfg.McpServers["serena"])
	require.NoError(t, err, "Failed to create MCP client for serena")
	defer mcpClient.Close()

	server, err := NewMCPServer("serena", cfg.McpProxy, cfg.McpServers["serena"])
	require.NoError(t, err, "Failed to create MCP server for serena")

	// Connect client to server
	err = mcpClient.AddToMCPServer(ctx, info, server.mcpServer)
	require.NoError(t, err, "Failed to add client to server for serena")

	// Register the handler
	httpMux.Handle("/serena/", server.handler)

	// Create a client to connect to our proxy
	proxyClient, err := client.NewStreamableHttpClient(testServer.URL + "/serena/")
	require.NoError(t, err, "Failed to create proxy client")
	defer proxyClient.Close()

	// Initialize the proxy client
	err = proxyClient.Start(ctx)
	require.NoError(t, err, "Failed to start proxy client")

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test-client"}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = proxyClient.Initialize(ctx, initRequest)
	require.NoError(t, err, "Failed to initialize proxy client")

	// Phase 1: Initial connection - should only see meta-tool
	t.Run("Initial connection shows only meta-tool", func(t *testing.T) {
		toolsRequest := mcp.ListToolsRequest{}
		toolsResponse, err := proxyClient.ListTools(ctx, toolsRequest)
		require.NoError(t, err, "Failed to list tools")

		// Should have exactly 1 meta-tool
		require.Len(t, toolsResponse.Tools, 1, "Should have exactly 1 meta-tool")

		metaTool := toolsResponse.Tools[0]

		// Verify meta-tool structure
		assert.Equal(t, "activate_serena", metaTool.Name, "Meta-tool name should be activate_serena")
		assert.NotEmpty(t, metaTool.Description, "Meta-tool should have a description")
		assert.Contains(t, metaTool.Description, "semantic", "Serena description should mention semantic code operations")

		t.Logf("Meta-tool: %s - %s", metaTool.Name, metaTool.Description)
	})

	// Phase 2: Activation - calling activate should load all real tools
	t.Run("Calling activate loads real tools", func(t *testing.T) {
		// Before activation: should have 1 meta-tool
		toolsRequest := mcp.ListToolsRequest{}
		toolsBefore, err := proxyClient.ListTools(ctx, toolsRequest)
		require.NoError(t, err)
		require.Len(t, toolsBefore.Tools, 1, "Should have 1 meta-tool before activation")

		metaTool := toolsBefore.Tools[0]

		// Call the activate tool
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = metaTool.Name
		callRequest.Params.Arguments = map[string]interface{}{}

		callResult, err := proxyClient.CallTool(ctx, callRequest)
		require.NoError(t, err, "Failed to call activate tool")
		require.NotNil(t, callResult, "Activate tool should return a result")

		// Verify activation response
		require.Len(t, callResult.Content, 1, "Should have activation response")

		textContent, ok := mcp.AsTextContent(callResult.Content[0])
		require.True(t, ok, "Response should be text content")

		var contentMap map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &contentMap)
		require.NoError(t, err, "Response should be valid JSON")

		assert.Equal(t, true, contentMap["activated"], "Response should indicate activation")
		assert.Equal(t, "serena", contentMap["server"], "Response should mention server name")

		toolCount, ok := contentMap["toolCount"].(float64)
		require.True(t, ok, "Should have tool count")
		t.Logf("Activated with %d tools", int(toolCount))

		// After activation: should have many real tools
		toolsAfter, err := proxyClient.ListTools(ctx, toolsRequest)
		require.NoError(t, err)

		// Serena has many tools (find_symbol, read_file, etc.)
		assert.Greater(t, len(toolsAfter.Tools), 5, "Should have multiple real tools after activation")

		// Verify we have actual Serena tools
		toolNames := make([]string, len(toolsAfter.Tools))
		for i, tool := range toolsAfter.Tools {
			toolNames[i] = tool.Name
		}

		assert.Contains(t, toolNames, "find_symbol", "Should have find_symbol tool")
		assert.Contains(t, toolNames, "get_symbols_overview", "Should have get_symbols_overview tool")

		t.Logf("Loaded %d tools after activation", len(toolsAfter.Tools))
	})
}

// TestLazyLoadingPlaywright tests with Playwright server
func TestLazyLoadingPlaywright(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := &config.Config{
		McpProxy: &config.MCPProxyConfigV2{
			BaseURL: "http://localhost",
			Addr:    ":0",
			Name:    "Test MCP Proxy",
			Version: "1.0.0",
			Type:    config.MCPServerTypeStreamable,
			Options: &config.OptionsV2{
				LazyLoad: optional.NewField(true),
			},
		},
		McpServers: map[string]*config.MCPClientConfigV2{
			"playwright": {
				TransportType: config.MCPClientTypeStdio,
				Command:       "npx",
				Args: []string{
					"@playwright/mcp@latest",
				},
				Env: map[string]string{},
				Options: &config.OptionsV2{
					LazyLoad: optional.NewField(true),
				},
			},
		},
	}

	// Start the HTTP server
	httpMux := http.NewServeMux()
	testServer := httptest.NewServer(httpMux)
	defer testServer.Close()

	info := mcp.Implementation{
		Name: cfg.McpProxy.Name,
	}

	// Initialize the Playwright client
	mcpClient, err := NewMCPClient("playwright", cfg.McpServers["playwright"])
	require.NoError(t, err, "Failed to create MCP client for playwright")
	defer mcpClient.Close()

	server, err := NewMCPServer("playwright", cfg.McpProxy, cfg.McpServers["playwright"])
	require.NoError(t, err, "Failed to create MCP server for playwright")

	err = mcpClient.AddToMCPServer(ctx, info, server.mcpServer)
	require.NoError(t, err, "Failed to add client to server for playwright")

	httpMux.Handle("/playwright/", server.handler)

	// Create a client to connect to our proxy
	proxyClient, err := client.NewStreamableHttpClient(testServer.URL + "/playwright/")
	require.NoError(t, err, "Failed to create proxy client")
	defer proxyClient.Close()

	err = proxyClient.Start(ctx)
	require.NoError(t, err, "Failed to start proxy client")

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test-client"}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = proxyClient.Initialize(ctx, initRequest)
	require.NoError(t, err, "Failed to initialize proxy client")

	t.Run("Playwright meta-tool and activation", func(t *testing.T) {
		// Check meta-tool
		toolsRequest := mcp.ListToolsRequest{}
		toolsBefore, err := proxyClient.ListTools(ctx, toolsRequest)
		require.NoError(t, err)
		require.Len(t, toolsBefore.Tools, 1, "Should have 1 meta-tool")

		metaTool := toolsBefore.Tools[0]
		assert.Equal(t, "activate_playwright", metaTool.Name)
		assert.Contains(t, metaTool.Description, "browser", "Playwright description should mention browser automation")

		// Activate
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = metaTool.Name
		callRequest.Params.Arguments = map[string]interface{}{}

		callResult, err := proxyClient.CallTool(ctx, callRequest)
		require.NoError(t, err, "Failed to activate Playwright")

		textContent, ok := mcp.AsTextContent(callResult.Content[0])
		require.True(t, ok)

		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		fmt.Printf("Playwright activation response: %+v\n", response)

		// Verify Playwright tools loaded
		toolsAfter, err := proxyClient.ListTools(ctx, toolsRequest)
		require.NoError(t, err)

		assert.Greater(t, len(toolsAfter.Tools), 1, "Should have multiple Playwright tools")

		toolNames := make([]string, len(toolsAfter.Tools))
		for i, tool := range toolsAfter.Tools {
			toolNames[i] = tool.Name
		}

		t.Logf("Loaded Playwright tools: %v", toolNames)

		// At least one should be playwright-related
		hasPlaywrightTool := false
		for _, name := range toolNames {
			if assert.Contains(t, name, "playwright") {
				hasPlaywrightTool = true
				break
			}
		}
		assert.True(t, hasPlaywrightTool, "Should have at least one playwright tool")
	})
}
