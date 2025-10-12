package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TBXark/optional-go"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecursiveProxyBasics tests the basic functionality of the recursive proxy
func TestRecursiveProxyBasics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test config
	config := &Config{
		McpProxy: &MCPProxyConfigV2{
			BaseURL: "http://localhost",
			Addr:    ":0",
			Name:    "Test Recursive Proxy",
			Version: "1.0.0",
			Type:    MCPServerTypeStreamable,
			Options: &OptionsV2{
				RecursiveLazyLoad: optional.NewField(true),
			},
		},
		McpServers: map[string]*MCPClientConfigV2{},
	}

	// Load hierarchy
	hierarchy, err := LoadHierarchy("testdata/mcp_hierarchy")
	require.NoError(t, err, "Failed to load hierarchy")

	// Create server registry
	registry := NewServerRegistry()
	defer registry.Close()

	// Create MCP server with meta-tools
	mcpServer := server.NewMCPServer(
		config.McpProxy.Name,
		config.McpProxy.Version,
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	)

	// Register get_tools_in_category meta-tool
	getToolsInCategoryTool := mcp.Tool{
		Name:        "get_tools_in_category",
		Description: "Navigate the tool hierarchy",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Category path",
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
		Description: "Execute a tool by path",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tool_path": map[string]interface{}{
					"type": "string",
				},
				"arguments": map[string]interface{}{
					"type": "object",
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

		return hierarchy.HandleExecuteTool(ctx, registry, toolPath, arguments)
	})

	// Create HTTP handler
	handler := server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true))
	httpMux := http.NewServeMux()
	httpMux.Handle("/", handler)

	// Start test server
	testServer := httptest.NewServer(httpMux)
	defer testServer.Close()

	// Create client to connect to the proxy
	proxyClient, err := client.NewStreamableHttpClient(testServer.URL + "/")
	require.NoError(t, err, "Failed to create proxy client")
	defer proxyClient.Close()

	// Start client
	err = proxyClient.Start(ctx)
	require.NoError(t, err, "Failed to start proxy client")

	// Initialize client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test-client"}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = proxyClient.Initialize(ctx, initRequest)
	require.NoError(t, err, "Failed to initialize proxy client")

	// Test 1: List tools - should see only 2 meta-tools
	t.Run("List tools returns only meta-tools", func(t *testing.T) {
		toolsRequest := mcp.ListToolsRequest{}
		toolsResponse, err := proxyClient.ListTools(ctx, toolsRequest)
		require.NoError(t, err, "Failed to list tools")

		require.Len(t, toolsResponse.Tools, 2, "Should have exactly 2 meta-tools")

		toolNames := make([]string, len(toolsResponse.Tools))
		for i, tool := range toolsResponse.Tools {
			toolNames[i] = tool.Name
		}

		assert.Contains(t, toolNames, "get_tools_in_category")
		assert.Contains(t, toolNames, "execute_tool")

		t.Logf("Meta-tools: %v", toolNames)
	})

	// Test 2: Call get_tools_in_category with empty path (root)
	t.Run("get_tools_in_category returns root structure", func(t *testing.T) {
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = "get_tools_in_category"
		callRequest.Params.Arguments = map[string]interface{}{
			"path": "",
		}

		callResult, err := proxyClient.CallTool(ctx, callRequest)
		require.NoError(t, err, "Failed to call get_tools_in_category")

		require.Len(t, callResult.Content, 1, "Should have response content")
		textContent, ok := mcp.AsTextContent(callResult.Content[0])
		require.True(t, ok, "Response should be text content")

		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err, "Response should be valid JSON")

		t.Logf("Root response: %+v", response)

		// Verify structure
		assert.Contains(t, response, "overview")
		assert.Contains(t, response, "categories")

		categories, ok := response["categories"].(map[string]interface{})
		require.True(t, ok, "Should have categories")
		assert.Contains(t, categories, "coding_tools")
		assert.Contains(t, categories, "web_tools")
	})

	// Test 3: Navigate to coding_tools
	t.Run("get_tools_in_category navigates to coding_tools", func(t *testing.T) {
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = "get_tools_in_category"
		callRequest.Params.Arguments = map[string]interface{}{
			"path": "coding_tools",
		}

		callResult, err := proxyClient.CallTool(ctx, callRequest)
		require.NoError(t, err, "Failed to navigate to coding_tools")

		textContent, ok := mcp.AsTextContent(callResult.Content[0])
		require.True(t, ok)

		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		t.Logf("coding_tools response: %+v", response)

		categories, ok := response["categories"].(map[string]interface{})
		require.True(t, ok, "Should have categories")

		// Should have serena and/or playwright
		t.Logf("Subcategories: %v", categories)
	})
}

// TestConfigLoading tests that the recursiveLazyLoad flag can be loaded from config
func TestConfigLoading(t *testing.T) {
	config, err := load("testdata/recursive_config_test.json", false, false, "", 0)
	require.NoError(t, err, "Failed to load config")

	assert.NotNil(t, config.McpProxy.Options)
	assert.True(t, config.McpProxy.Options.RecursiveLazyLoad.OrElse(false), "recursiveLazyLoad should be true")

	t.Logf("Config loaded successfully with recursiveLazyLoad=%v", config.McpProxy.Options.RecursiveLazyLoad.OrElse(false))
}
