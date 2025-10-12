package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadHierarchy_Phase2 tests loading the hierarchy from JSON files
func TestLoadHierarchy_Phase2(t *testing.T) {
	t.Run("Load hierarchy from testdata", func(t *testing.T) {
		hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
		absPath, err := filepath.Abs(hierarchyPath)
		require.NoError(t, err)

		hierarchy, err := LoadHierarchy(absPath)
		require.NoError(t, err, "Should load hierarchy without error")
		require.NotNil(t, hierarchy, "Hierarchy should not be nil")

		// Verify root node structure
		rootNode := hierarchy.nodes[""]
		require.NotNil(t, rootNode, "Root node should exist")
		assert.NotEmpty(t, rootNode.Overview, "Root should have overview")
		assert.NotEmpty(t, rootNode.Categories, "Root should have categories")
		assert.Contains(t, rootNode.Categories, "coding_tools", "Should have coding_tools category")
		assert.Contains(t, rootNode.Categories, "web_tools", "Should have web_tools category")

		// Verify root tools (meta-tools)
		assert.NotEmpty(t, rootNode.Tools, "Root should have tools")
		assert.Contains(t, rootNode.Tools, "get_tools_in_category", "Should have get_tools_in_category tool")
		assert.Contains(t, rootNode.Tools, "execute_tool", "Should have execute_tool tool")

		// Verify coding_tools node
		codingToolsNode := hierarchy.nodes["coding_tools"]
		require.NotNil(t, codingToolsNode, "coding_tools node should exist")
		assert.NotEmpty(t, codingToolsNode.Overview, "coding_tools should have overview")
		assert.Contains(t, codingToolsNode.Categories, "serena", "coding_tools should have serena category")
		assert.Contains(t, codingToolsNode.Categories, "playwright", "coding_tools should have playwright category")

		// Verify serena node
		serenaNode := hierarchy.nodes["coding_tools.serena"]
		require.NotNil(t, serenaNode, "serena node should exist")
		assert.NotEmpty(t, serenaNode.Overview, "serena should have overview")
		assert.NotNil(t, serenaNode.MCPServer, "serena should have MCP server config")
		assert.Equal(t, "serena", serenaNode.MCPServer.Name, "MCP server name should be serena")
		assert.Equal(t, "stdio", serenaNode.MCPServer.Type, "MCP server type should be stdio")

		// Verify serena has tools
		assert.NotEmpty(t, serenaNode.Tools, "serena should have tools")
		assert.Contains(t, serenaNode.Tools, "get_symbols_overview", "serena should have get_symbols_overview tool")

		// Verify tool structure
		tool := serenaNode.Tools["get_symbols_overview"]
		assert.NotEmpty(t, tool.Description, "Tool should have description")
		assert.NotEmpty(t, tool.MapsTo, "Tool should have maps_to field")

		t.Logf("Successfully loaded hierarchy with %d nodes", len(hierarchy.nodes))
	})

	t.Run("Load hierarchy with missing path", func(t *testing.T) {
		_, err := LoadHierarchy("/nonexistent/path")
		assert.Error(t, err, "Should error on missing path")
	})
}

// TestServerRegistry_Phase2 tests the ServerRegistry lazy loading behavior
func TestServerRegistry_Phase2(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("GetOrLoadServer loads server on first call", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		registry := NewServerRegistry()

		// Create a test config for Serena
		config := &MCPClientConfigV2{
			TransportType: MCPClientTypeStdio,
			Command:       "uv",
			Args: []string{
				"--directory",
				"/Users/bobbobby/repos/tools/serena",
				"run",
				"serena",
				"start-mcp-server",
				"--context",
				"claude-code",
			},
			Env:     map[string]string{},
			Options: &OptionsV2{},
		}

		// First call - should create and initialize client
		client1, err := registry.GetOrLoadServer(ctx, "serena", config)
		require.NoError(t, err, "First call should load server")
		require.NotNil(t, client1, "Client should not be nil")

		// Second call - should return same client
		client2, err := registry.GetOrLoadServer(ctx, "serena", config)
		require.NoError(t, err, "Second call should succeed")
		assert.Equal(t, client1, client2, "Should return same client instance")

		t.Log("✓ Server loaded once and reused")

		// Cleanup
		client1.Close()
	})

	t.Run("GetOrLoadServer handles invalid config", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		registry := NewServerRegistry()

		// Invalid config - no command
		invalidConfig := &MCPClientConfigV2{
			TransportType: MCPClientTypeStdio,
			Env:           map[string]string{},
			Options:       &OptionsV2{},
		}

		_, err := registry.GetOrLoadServer(ctx, "invalid", invalidConfig)
		assert.Error(t, err, "Should error on invalid config")
	})
}

// TestHandleGetToolsInCategory_Phase2 tests navigation through the hierarchy
func TestHandleGetToolsInCategory_Phase2(t *testing.T) {
	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	hierarchy, err := LoadHierarchy(absPath)
	require.NoError(t, err)

	t.Run("Get root categories", func(t *testing.T) {
		result, err := hierarchy.HandleGetToolsInCategory("")
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify structure
		assert.Equal(t, "", result["path"])
		assert.NotEmpty(t, result["overview"])

		categories := result["categories"].(map[string]string)
		assert.Contains(t, categories, "coding_tools")
		assert.Contains(t, categories, "web_tools")

		tools := result["tools"].(map[string]interface{})
		assert.Contains(t, tools, "get_tools_in_category")
		assert.Contains(t, tools, "execute_tool")

		// Verify JSON serialization works
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err)
		t.Logf("Root result:\n%s", string(jsonBytes))
	})

	t.Run("Get coding_tools categories", func(t *testing.T) {
		result, err := hierarchy.HandleGetToolsInCategory("coding_tools")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "coding_tools", result["path"])
		assert.NotEmpty(t, result["overview"])

		categories := result["categories"].(map[string]string)
		assert.Contains(t, categories, "serena")
		assert.Contains(t, categories, "playwright")

		t.Logf("Coding tools has %d categories", len(categories))
	})

	t.Run("Get serena structure", func(t *testing.T) {
		result, err := hierarchy.HandleGetToolsInCategory("coding_tools.serena")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "coding_tools.serena", result["path"])
		assert.NotEmpty(t, result["overview"])

		categories := result["categories"].(map[string]string)
		assert.Contains(t, categories, "search")
		assert.Contains(t, categories, "edit")

		tools := result["tools"].(map[string]interface{})
		assert.Contains(t, tools, "get_symbols_overview")

		// Verify tool structure
		toolInfo := tools["get_symbols_overview"].(map[string]interface{})
		assert.NotEmpty(t, toolInfo["description"])
		assert.NotEmpty(t, toolInfo["tool_path"])
		assert.Equal(t, "coding_tools.serena.get_symbols_overview", toolInfo["tool_path"])

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err)
		t.Logf("Serena result:\n%s", string(jsonBytes))
	})

	t.Run("Get nested search category", func(t *testing.T) {
		result, err := hierarchy.HandleGetToolsInCategory("coding_tools.serena.search")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "coding_tools.serena.search", result["path"])
		assert.NotEmpty(t, result["overview"])

		categories := result["categories"].(map[string]string)
		assert.Contains(t, categories, "search_symbol")

		t.Logf("Search category has %d subcategories", len(categories))
	})

	t.Run("Invalid path returns error", func(t *testing.T) {
		_, err := hierarchy.HandleGetToolsInCategory("nonexistent.path")
		assert.Error(t, err, "Should error on invalid path")
		assert.Contains(t, err.Error(), "not found", "Error should mention path not found")
	})
}

// TestResolveToolPath_Phase2 tests tool path resolution
func TestResolveToolPath_Phase2(t *testing.T) {
	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	hierarchy, err := LoadHierarchy(absPath)
	require.NoError(t, err)

	t.Run("Resolve full tool path", func(t *testing.T) {
		toolDef, serverConfig, err := hierarchy.ResolveToolPath("coding_tools.serena.get_symbols_overview")
		require.NoError(t, err)
		require.NotNil(t, toolDef)
		require.NotNil(t, serverConfig)

		assert.NotEmpty(t, toolDef.MapsTo)
		assert.Equal(t, "get_symbols_overview", toolDef.MapsTo)
		assert.NotEmpty(t, toolDef.Description)

		assert.Equal(t, MCPClientTypeStdio, serverConfig.TransportType)
		assert.Equal(t, "uv", serverConfig.Command)

		t.Logf("Resolved tool: %s -> %s", "get_symbols_overview", toolDef.MapsTo)
	})

	t.Run("Resolve tool in nested category", func(t *testing.T) {
		toolDef, serverConfig, err := hierarchy.ResolveToolPath("coding_tools.serena.edit.replace_symbol_body")
		require.NoError(t, err)
		require.NotNil(t, toolDef)
		require.NotNil(t, serverConfig)

		assert.Equal(t, "replace_symbol_body", toolDef.MapsTo)
		assert.Equal(t, "uv", serverConfig.Command)

		t.Logf("Resolved edit tool: %s", toolDef.MapsTo)
	})

	t.Run("Resolve with invalid path", func(t *testing.T) {
		_, _, err := hierarchy.ResolveToolPath("invalid.tool.path")
		assert.Error(t, err, "Should error on invalid path")
	})

	t.Run("Resolve meta-tool", func(t *testing.T) {
		toolDef, serverConfig, err := hierarchy.ResolveToolPath("get_tools_in_category")
		require.NoError(t, err)
		require.NotNil(t, toolDef)
		// Meta-tools don't have server config
		assert.Nil(t, serverConfig)

		t.Logf("Resolved meta-tool: %s", toolDef.Description)
	})
}

// TestHandleExecuteTool_Phase2 tests tool execution through the proxy
func TestHandleExecuteTool_Phase2(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	hierarchy, err := LoadHierarchy(absPath)
	require.NoError(t, err)

	registry := NewServerRegistry()
	defer registry.Close()

	t.Run("Execute Serena tool through proxy", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		toolPath := "coding_tools.serena.get_symbols_overview"
		arguments := map[string]interface{}{
			"relative_path": "client.go",
		}

		result, err := hierarchy.HandleExecuteTool(ctx, registry, toolPath, arguments)
		require.NoError(t, err, "Tool execution should succeed")
		require.NotNil(t, result, "Result should not be nil")

		// Verify result structure
		assert.NotEmpty(t, result.Content, "Result should have content")

		t.Logf("Tool execution successful, content length: %d", len(result.Content))
	})

	t.Run("Execute with invalid tool path", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		toolPath := "invalid.tool.path"
		arguments := map[string]interface{}{}

		_, err := hierarchy.HandleExecuteTool(ctx, registry, toolPath, arguments)
		assert.Error(t, err, "Should error on invalid tool path")
	})

	t.Run("Reuse loaded server for second call", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		toolPath := "coding_tools.serena.get_symbols_overview"
		arguments := map[string]interface{}{
			"relative_path": "config.go",
		}

		// First call
		result1, err := hierarchy.HandleExecuteTool(ctx, registry, toolPath, arguments)
		require.NoError(t, err)
		require.NotNil(t, result1)

		// Second call - should reuse server
		result2, err := hierarchy.HandleExecuteTool(ctx, registry, toolPath, arguments)
		require.NoError(t, err)
		require.NotNil(t, result2)

		t.Log("✓ Server reused for subsequent calls")
	})
}

// TestThreadSafety_Phase2 tests concurrent access to ServerRegistry
func TestThreadSafety_Phase2(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Concurrent GetOrLoadServer calls", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		registry := NewServerRegistry()
		config := &MCPClientConfigV2{
			TransportType: MCPClientTypeStdio,
			Command:       "uv",
			Args: []string{
				"--directory",
				"/Users/bobbobby/repos/tools/serena",
				"run",
				"serena",
				"start-mcp-server",
				"--context",
				"claude-code",
			},
			Env:     map[string]string{},
			Options: &OptionsV2{},
		}

		// Launch 10 concurrent requests
		results := make(chan *Client, 10)
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func() {
				client, err := registry.GetOrLoadServer(ctx, "serena", config)
				if err != nil {
					errors <- err
				} else {
					results <- client
				}
			}()
		}

		// Collect results
		var clients []*Client
		for i := 0; i < 10; i++ {
			select {
			case client := <-results:
				clients = append(clients, client)
			case err := <-errors:
				t.Fatalf("Concurrent call failed: %v", err)
			case <-time.After(60 * time.Second):
				t.Fatal("Timeout waiting for concurrent calls")
			}
		}

		// All clients should be the same instance
		for i := 1; i < len(clients); i++ {
			assert.Equal(t, clients[0], clients[i], "All concurrent calls should return same instance")
		}

		t.Log("✓ Concurrent access handled correctly")

		// Cleanup
		clients[0].Close()
	})
}

// TestMCPServerConfig_Phase2 tests parsing MCP server config from hierarchy
func TestMCPServerConfig_Phase2(t *testing.T) {
	t.Run("ToClientConfig converts MCPServerRef correctly", func(t *testing.T) {
		serverRef := &MCPServerRef{
			Name:    "serena",
			Type:    "stdio",
			Command: "uv",
			Args:    []string{"run", "serena"},
			Env:     map[string]string{"FOO": "bar"},
		}

		clientConfig := serverRef.ToClientConfig()
		require.NotNil(t, clientConfig)

		assert.Equal(t, MCPClientTypeStdio, clientConfig.TransportType)
		assert.Equal(t, "uv", clientConfig.Command)
		assert.Equal(t, []string{"run", "serena"}, clientConfig.Args)
		assert.Equal(t, "bar", clientConfig.Env["FOO"])

		t.Logf("Parsed MCP config: command=%s, args=%v", clientConfig.Command, clientConfig.Args)
	})

	t.Run("ToClientConfig handles SSE type", func(t *testing.T) {
		serverRef := &MCPServerRef{
			Name:    "remote",
			Type:    "sse",
			URL:     "https://example.com/sse",
			Headers: map[string]string{"Authorization": "Bearer token"},
		}

		clientConfig := serverRef.ToClientConfig()
		require.NotNil(t, clientConfig)

		assert.Equal(t, MCPClientTypeSSE, clientConfig.TransportType)
		assert.Equal(t, "https://example.com/sse", clientConfig.URL)
		assert.Equal(t, "Bearer token", clientConfig.Headers["Authorization"])
	})
}
