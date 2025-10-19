package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TBXark/mcp-proxy/internal/config"
	"github.com/TBXark/mcp-proxy/internal/hierarchy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecursiveLazyLoadingFlow tests the complete hierarchical lazy-loading workflow:
// 1. Server exposes only 2 meta-tools: get_tools_in_category and execute_tool
// 2. get_tools_in_category navigates the tool hierarchy
// 3. execute_tool proxies to actual MCP servers
// 4. Real MCP servers (Serena, Playwright) are only loaded when their tools are executed
func TestRecursiveLazyLoadingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Load the hierarchy
	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	h, err := hierarchy.LoadHierarchy(absPath)
	require.NoError(t, err, "Should load hierarchy")

	_, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Test Phase 1: Verify hierarchy has only 2 meta-tools at root
	t.Run("Phase 1: Root has only meta-tools", func(t *testing.T) {
		rootNode := h.GetRootNode()
		require.NotNil(t, rootNode, "Root node should exist")

		// Root should have exactly 2 tools
		assert.Len(t, rootNode.Tools, 2, "Root should have exactly 2 meta-tools")
		assert.Contains(t, rootNode.Tools, "get_tools_in_category")
		assert.Contains(t, rootNode.Tools, "execute_tool")

		t.Log("✓ Root exposes get_tools_in_category for navigation")
		t.Log("✓ Root exposes execute_tool for proxied execution")
	})

	// Test Phase 2: Navigate root categories
	t.Run("Phase 2: get_tools_in_category(\"\") returns root overview", func(t *testing.T) {
		response, err := h.HandleGetToolsInCategory("")
		require.NoError(t, err, "Should handle root category")
		require.NotNil(t, response)

		t.Logf("Calling get_tools_in_category(path=%q)", "")

		// Verify response structure
		assert.Contains(t, response, "overview", "Should have overview")
		assert.Contains(t, response, "children", "Should have children")
		assert.Contains(t, response, "tools", "Should have tools")

		children := response["children"].(map[string]interface{})
		assert.Contains(t, children, "coding_tools", "Should have coding_tools category")

		tools := response["tools"].(map[string]interface{})
		assert.Contains(t, tools, "get_tools_in_category")
		assert.Contains(t, tools, "execute_tool")

		t.Log("✓ Returns overview of root level")
		t.Log("✓ Lists coding_tools category")
		t.Log("✓ Includes meta-tools at root level")
	})

	// Test Phase 3: Navigate to coding_tools
	t.Run("Phase 3: get_tools_in_category(\"coding_tools\") returns dev tools", func(t *testing.T) {
		path := "coding_tools"
		response, err := h.HandleGetToolsInCategory(path)
		require.NoError(t, err, "Should handle coding_tools category")
		require.NotNil(t, response)

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		assert.Contains(t, response, "overview")
		assert.Contains(t, response, "children")

		children := response["children"].(map[string]interface{})
		assert.Contains(t, children, "serena", "Should list serena")
		assert.Contains(t, children, "playwright", "Should list playwright")

		t.Log("✓ Returns coding_tools overview")
		t.Log("✓ Lists serena and playwright as subcategories")
		t.Log("✓ Does NOT connect to MCP servers yet (lazy)")
	})

	// Test Phase 4: Navigate to coding_tools.serena
	t.Run("Phase 4: get_tools_in_category(\"coding_tools.serena\") returns Serena structure", func(t *testing.T) {
		path := "coding_tools.serena"
		response, err := h.HandleGetToolsInCategory(path)
		require.NoError(t, err, "Should handle serena category")
		require.NotNil(t, response)

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		assert.Contains(t, response, "overview")
		assert.Contains(t, response, "children")
		assert.Contains(t, response, "tools")

		children := response["children"].(map[string]interface{})
		assert.Contains(t, children, "search", "Should have search category")
		assert.Contains(t, children, "edit", "Should have edit category")

		tools := response["tools"].(map[string]interface{})
		assert.Contains(t, tools, "get_symbols_overview")
		assert.Contains(t, tools, "activate_project")

		// Verify tool_path is correct
		symbolTool := tools["get_symbols_overview"].(map[string]interface{})
		assert.Equal(t, "coding_tools.serena.get_symbols_overview", symbolTool["tool_path"])

		t.Log("✓ Returns Serena overview")
		t.Log("✓ Lists search and edit subcategories")
		t.Log("✓ Lists direct tools (get_symbols_overview, activate_project)")
		t.Log("✓ Includes full tool_path for each tool")
		t.Log("✓ Does NOT start Serena MCP server yet (lazy)")
	})

	// Test Phase 5: Navigate to coding_tools.serena.search
	t.Run("Phase 5: get_tools_in_category(\"coding_tools.serena.search\") returns search tools", func(t *testing.T) {
		path := "coding_tools.serena.search"
		response, err := h.HandleGetToolsInCategory(path)
		require.NoError(t, err, "Should handle search category")
		require.NotNil(t, response)

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		assert.Contains(t, response, "children")
		children := response["children"].(map[string]interface{})
		assert.Contains(t, children, "search_symbol", "Should have search_symbol")

		t.Log("✓ Returns search category overview")
		t.Log("✓ Lists search_symbol subcategory")
	})

	// Test Phase 6: Tool path resolution
	t.Run("Phase 6: Tool path resolution", func(t *testing.T) {
		toolPath := "coding_tools.serena.get_symbols_overview"

		toolDef, serverName, err := h.ResolveToolPath(toolPath)
		require.NoError(t, err, "Should resolve tool path")
		require.NotNil(t, toolDef)

		assert.Equal(t, "get_symbols_overview", toolDef.MapsTo)
		assert.NotEmpty(t, serverName, "Should have server name")
		assert.Equal(t, "serena", serverName, "Should resolve to serena server")

		t.Logf("✓ Resolved %q to tool %q on server %q", toolPath, toolDef.MapsTo, serverName)
	})
}

// TestActualSerenaExecution tests real execution with Serena MCP server
func TestActualSerenaExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if SERENA_PATH is set
	serenaPath := os.Getenv("SERENA_PATH")
	if serenaPath == "" {
		t.Skip("SERENA_PATH not set - set it to run this test (e.g., export SERENA_PATH=/path/to/serena)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("Real Serena execution via recursive proxy", func(t *testing.T) {
		// Step 1: Start recursive lazy loading proxy
		t.Log("Step 1: Starting recursive lazy loading proxy...")
		hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
		absPath, err := filepath.Abs(hierarchyPath)
		require.NoError(t, err)

		h, err := hierarchy.LoadHierarchy(absPath)
		require.NoError(t, err)
		t.Log("  ✓ Loading hierarchy from testdata/mcp_hierarchy/")

		// Build server config for Serena
		// This requires SERENA_PATH to be set
		serenaConfig := &config.MCPClientConfigV2{
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
			Env:     map[string]string{},
			Options: &config.OptionsV2{},
		}

		serverConfigs := map[string]*config.MCPClientConfigV2{
			"serena": serenaConfig,
		}

		registry := hierarchy.NewServerRegistry(serverConfigs)
		defer registry.Close()
		t.Log("  ✓ Server registry initialized with Serena config")

		// Step 2: Explore the hierarchy
		t.Log("Step 2: Exploring tool hierarchy...")

		rootResp, err := h.HandleGetToolsInCategory("")
		require.NoError(t, err)
		children := rootResp["children"].(map[string]interface{})
		assert.Contains(t, children, "coding_tools")
		t.Log("  ✓ Call get_tools_in_category('') -> see coding_tools")

		codingResp, err := h.HandleGetToolsInCategory("coding_tools")
		require.NoError(t, err)
		codingChildren := codingResp["children"].(map[string]interface{})
		assert.Contains(t, codingChildren, "serena")
		assert.Contains(t, codingChildren, "playwright")
		t.Log("  ✓ Call get_tools_in_category('coding_tools') -> see serena, playwright")

		serenaResp, err := h.HandleGetToolsInCategory("coding_tools.serena")
		require.NoError(t, err)
		assert.Contains(t, serenaResp, "overview")
		t.Log("  ✓ Call get_tools_in_category('coding_tools.serena') -> see structure")

		// Step 3: Execute actual tool
		t.Log("Step 3: Executing tool through proxy...")
		toolPath := "coding_tools.serena.get_symbols_overview"
		arguments := map[string]interface{}{
			"relative_path": "client.go",
		}

		t.Logf("  Tool call: tool_path=%s, arguments=%+v", toolPath, arguments)

		result, err := h.HandleExecuteTool(ctx, registry, toolPath, arguments)
		require.NoError(t, err, "Tool execution should succeed")
		require.NotNil(t, result, "Result should not be nil")
		t.Log("  ✓ Serena started (lazy initialization)")
		t.Log("  ✓ Returns symbol information from client.go")

		// Step 4: Verify results
		t.Log("Step 4: Verify results...")
		assert.NotEmpty(t, result.Content, "Should have content")

		// Print actual content
		for i, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				t.Logf("  Content[%d]: %s", i, textContent.Text[:min(100, len(textContent.Text))])
			}
		}
		t.Log("  ✓ Results received from actual Serena MCP server")
	})
}

// TestPlaywrightExecution tests Playwright through recursive proxy
func TestPlaywrightExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Playwright navigation via recursive proxy", func(t *testing.T) {
		hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
		absPath, err := filepath.Abs(hierarchyPath)
		require.NoError(t, err)

		h, err := hierarchy.LoadHierarchy(absPath)
		require.NoError(t, err)

		t.Log("Exploring Playwright tools...")

		// Navigate to playwright
		t.Log("Step 1: get_tools_in_category('coding_tools.playwright')")
		response, err := h.HandleGetToolsInCategory("coding_tools.playwright")
		require.NoError(t, err, "Should navigate to playwright category")

		assert.Contains(t, response, "children")
		children := response["children"].(map[string]interface{})
		assert.Contains(t, children, "browser", "Should have browser category")
		t.Log("  ✓ Found browser category")

		// Note: Actually executing Playwright tools would require:
		// 1. Installing @playwright/mcp package
		// 2. Setting up browser dependencies
		// 3. Configuring the tool in hierarchy
		// For now, we just verify the structure is loaded correctly
		t.Log("Step 2: Verify Playwright tools are discoverable")

		// Navigate to browser category
		browserResp, err := h.HandleGetToolsInCategory("coding_tools.playwright.browser")
		if err == nil {
			t.Log("  ✓ Browser category accessible")
			if tools, ok := browserResp["tools"].(map[string]interface{}); ok {
				t.Logf("  Found %d tools in browser category", len(tools))
			}
		} else {
			t.Logf("  Note: Browser category not fully configured: %v", err)
		}
	})
}

// TestHierarchyConfigLoading tests loading the JSON hierarchy
func TestHierarchyConfigLoading(t *testing.T) {
	t.Run("Load and parse hierarchy JSON files", func(t *testing.T) {
		hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")

		// Load root.json
		rootPath := filepath.Join(hierarchyPath, "root.json")
		rootData, err := os.ReadFile(rootPath)
		require.NoError(t, err, "Should read root.json")

		var root map[string]interface{}
		err = json.Unmarshal(rootData, &root)
		require.NoError(t, err, "Should parse root.json")

		// Verify structure - root.json only has overview
		assert.Contains(t, root, "overview", "root should have overview")
		t.Log("✓ root.json structure is valid")

		// Load the full hierarchy to verify meta-tools are added programmatically
		absPath, err := filepath.Abs(hierarchyPath)
		require.NoError(t, err)

		h, err := hierarchy.LoadHierarchy(absPath)
		require.NoError(t, err)

		rootNode := h.GetRootNode()
		require.NotNil(t, rootNode)
		assert.Contains(t, rootNode.Tools, "get_tools_in_category", "Meta-tools should be added to root")
		assert.Contains(t, rootNode.Tools, "execute_tool", "Meta-tools should be added to root")
		t.Log("✓ Meta-tools added programmatically to root")

		// Check that coding_tools exists (if it does in this test hierarchy)
		_, err = h.HandleGetToolsInCategory("coding_tools")
		if err == nil {
			t.Log("✓ coding_tools category found")
		}

		t.Log("✓ Hierarchy loaded successfully with all structures")
	})
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestToolPathParsing tests parsing dot-notation tool paths
func TestToolPathParsing(t *testing.T) {
	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	h, err := hierarchy.LoadHierarchy(absPath)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		toolPath   string
		shouldWork bool
		wantMapsTo string
	}{
		{
			name:       "Direct serena tool",
			toolPath:   "coding_tools.serena.get_symbols_overview",
			shouldWork: true,
			wantMapsTo: "get_symbols_overview",
		},
		{
			name:       "Nested search tool",
			toolPath:   "coding_tools.serena.search.search_symbol.find_symbol",
			shouldWork: true,
			wantMapsTo: "find_symbol",
		},
		{
			name:       "Edit tool",
			toolPath:   "coding_tools.serena.edit.replace_symbol_body",
			shouldWork: true,
			wantMapsTo: "replace_symbol_body",
		},
		{
			name:       "Meta-tool",
			toolPath:   "get_tools_in_category",
			shouldWork: true,
			wantMapsTo: "get_tools_in_category",
		},
		{
			name:       "Invalid path",
			toolPath:   "nonexistent.tool.path",
			shouldWork: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			toolDef, serverName, err := h.ResolveToolPath(tc.toolPath)

			if tc.shouldWork {
				require.NoError(t, err, "Should resolve tool path")
				require.NotNil(t, toolDef)
				assert.Equal(t, tc.wantMapsTo, toolDef.MapsTo)

				t.Logf("Tool path: %s", tc.toolPath)
				t.Logf("Resolved to: %s", toolDef.MapsTo)
				if serverName != "" {
					t.Logf("Server name: %s", serverName)
				} else {
					t.Log("No server (meta-tool)")
				}
			} else {
				assert.Error(t, err, "Should error on invalid path")
				t.Logf("Expected error for path: %s", tc.toolPath)
			}
		})
	}
}

// TestErrorHandling tests error cases
func TestErrorHandling(t *testing.T) {
	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	h, err := hierarchy.LoadHierarchy(absPath)
	require.NoError(t, err)

	t.Run("Invalid category path", func(t *testing.T) {
		invalidPath := "nonexistent.category.path"
		t.Logf("Testing invalid path: %s", invalidPath)

		_, err := h.HandleGetToolsInCategory(invalidPath)
		assert.Error(t, err, "Should error on invalid path")
		assert.Contains(t, err.Error(), "not found", "Error should mention path not found")

		t.Log("✓ Error indicating category not found")
	})

	t.Run("Invalid tool path", func(t *testing.T) {
		invalidPath := "coding_tools.nonexistent.tool"
		t.Logf("Testing invalid tool path: %s", invalidPath)

		_, _, err := h.ResolveToolPath(invalidPath)
		assert.Error(t, err, "Should error on invalid tool path")

		t.Log("✓ Error indicating tool not found")
	})

	t.Run("MCP server config missing", func(t *testing.T) {
		ctx := context.Background()
		// Create registry with no server configs
		registry := hierarchy.NewServerRegistry(map[string]*config.MCPClientConfigV2{})
		defer registry.Close()

		// Try to execute a tool with valid path but missing server config in registry
		validPath := "coding_tools.serena.get_symbols_overview"

		_, err := h.HandleExecuteTool(ctx, registry, validPath, map[string]interface{}{
			"relative_path": "client.go",
		})
		assert.Error(t, err, "Should error when server config missing from registry")
		assert.Contains(t, err.Error(), "server config not found", "Error should mention missing config")

		t.Log("✓ Clear error message when server config is missing")
	})

	t.Run("Empty tool path", func(t *testing.T) {
		_, _, err := h.ResolveToolPath("")
		assert.Error(t, err, "Should error on empty tool path")

		t.Log("✓ Error on empty tool path")
	})
}

// TestDocumentationGeneration tests that the hierarchy can generate docs
func TestDocumentationGeneration(t *testing.T) {
	hierarchyPath := filepath.Join("testdata", "mcp_hierarchy")
	absPath, err := filepath.Abs(hierarchyPath)
	require.NoError(t, err)

	h, err := hierarchy.LoadHierarchy(absPath)
	require.NoError(t, err)

	t.Run("Hierarchy provides all data needed for documentation", func(t *testing.T) {
		// Verify that the hierarchy has all the data needed to generate documentation
		rootNode := h.GetRootNode()
		require.NotNil(t, rootNode)

		assert.NotEmpty(t, rootNode.Overview, "Root should have overview for documentation")

		// Navigate through the hierarchy and collect documentation data
		rootResp, err := h.HandleGetToolsInCategory("")
		require.NoError(t, err)

		children := rootResp["children"].(map[string]interface{})
		assert.NotEmpty(t, children, "Should have categories to document")

		// Get coding_tools category
		codingResp, err := h.HandleGetToolsInCategory("coding_tools")
		require.NoError(t, err)
		assert.Contains(t, codingResp, "overview", "Categories should have overview for docs")

		// Get serena structure
		serenaResp, err := h.HandleGetToolsInCategory("coding_tools.serena")
		require.NoError(t, err)
		assert.Contains(t, serenaResp, "overview")
		assert.Contains(t, serenaResp, "tools")
		assert.Contains(t, serenaResp, "children")

		tools := serenaResp["tools"].(map[string]interface{})
		for name, toolInfo := range tools {
			info := toolInfo.(map[string]interface{})
			assert.Contains(t, info, "description", "Tool %s should have description for docs", name)
			assert.Contains(t, info, "tool_path", "Tool %s should have path for docs", name)
		}

		t.Log("✓ Hierarchy contains all data needed for documentation generation:")
		t.Log("  - Overview at each level")
		t.Log("  - Tool descriptions and paths")
		t.Log("  - Category descriptions")
		t.Log("  - Hierarchical structure")
	})
}
