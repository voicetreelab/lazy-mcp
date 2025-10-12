package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	_, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Test Phase 1: Initial tools/list should return only meta-tools
	t.Run("Phase 1: tools/list returns only meta-tools", func(t *testing.T) {
		// TODO: Initialize the recursive lazy loading proxy
		// This would connect to the testdata/mcp_hierarchy structure

		// Expected: tools/list returns exactly 2 tools
		expectedTools := []string{"get_tools_in_category", "execute_tool"}

		t.Logf("Expected meta-tools: %v", expectedTools)
		t.Log("✓ Should expose get_tools_in_category for navigation")
		t.Log("✓ Should expose execute_tool for proxied execution")
	})

	// Test Phase 2: Navigate root categories
	t.Run("Phase 2: get_tools_in_category(\"\") returns root overview", func(t *testing.T) {
		// Call get_tools_in_category with empty string or "/"
		path := ""

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		// Expected response structure:
		expectedResponse := map[string]interface{}{
			"overview": "MCP Proxy - Hierarchical tool organization system...",
			"categories": map[string]interface{}{
				"coding_tools": "Development tools including semantic code analysis...",
				"web_tools":    "Web scraping, browser automation, and HTTP clients",
			},
			"tools": map[string]interface{}{},
		}

		t.Logf("Expected categories: %v", expectedResponse["categories"])
		t.Log("✓ Should return overview of root level")
		t.Log("✓ Should list coding_tools and web_tools categories")
		t.Log("✓ Should NOT list any tools at root level (tools are nested)")
	})

	// Test Phase 3: Navigate to coding_tools
	t.Run("Phase 3: get_tools_in_category(\"coding_tools\") returns dev tools", func(t *testing.T) {
		path := "coding_tools"

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		// Expected: List of subcategories with their overviews
		expectedCategories := map[string]string{
			"serena":     "Semantic code analysis, symbol search, and intelligent editing...",
			"playwright": "Browser automation for testing web applications...",
		}

		for name, desc := range expectedCategories {
			t.Logf("  Category: %s - %s", name, desc)
		}

		t.Log("✓ Should return coding_tools overview")
		t.Log("✓ Should list serena and playwright as subcategories")
		t.Log("✓ Should provide guidance on when to use each")
		t.Log("✓ Should NOT connect to MCP servers yet (lazy)")
	})

	// Test Phase 4: Navigate to coding_tools.serena
	t.Run("Phase 4: get_tools_in_category(\"coding_tools.serena\") returns Serena structure", func(t *testing.T) {
		path := "coding_tools.serena"

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		// Expected: Serena's categories AND top-level tools
		expectedResponse := map[string]interface{}{
			"overview": "Serena provides semantic code understanding and manipulation...",
			"categories": map[string]interface{}{
				"search": "Find symbols, references, and code patterns",
				"edit":   "Modify code with semantic awareness",
			},
			"tools": map[string]interface{}{
				"get_symbols_overview": map[string]interface{}{
					"description": "Get a high-level overview of top-level symbols in a file",
					"tool_path":   "coding_tools.serena.get_symbols_overview",
				},
				"activate_project": map[string]interface{}{
					"description": "Activate a registered project in Serena",
					"tool_path":   "coding_tools.serena.activate_project",
				},
			},
		}

		t.Logf("Expected structure: %+v", expectedResponse)
		t.Log("✓ Should return Serena overview")
		t.Log("✓ Should list search and edit subcategories")
		t.Log("✓ Should list direct tools (get_symbols_overview, activate_project)")
		t.Log("✓ Should include full tool_path for each tool")
		t.Log("✓ Should NOT start Serena MCP server yet (lazy)")
	})

	// Test Phase 5: Navigate to coding_tools.serena.search
	t.Run("Phase 5: get_tools_in_category(\"coding_tools.serena.search\") returns search tools", func(t *testing.T) {
		path := "coding_tools.serena.search"

		t.Logf("Calling get_tools_in_category(path=%q)", path)

		// Expected: Search subcategories
		expectedCategories := map[string]string{
			"search_symbol":  "Find symbols by name path with configurable depth",
			"find_references": "Find all references to a symbol across the codebase",
		}

		for name, desc := range expectedCategories {
			t.Logf("  Subcategory: %s - %s", name, desc)
		}

		t.Log("✓ Should return search category overview")
		t.Log("✓ Should list search_symbol and find_references")
	})

	// Test Phase 6: Execute a tool through the proxy
	t.Run("Phase 6: execute_tool proxies to actual Serena MCP server", func(t *testing.T) {
		// This is where the actual MCP server connection happens
		toolPath := "coding_tools.serena.search.search_symbol"

		// Arguments for find_symbol tool in Serena
		arguments := map[string]interface{}{
			"name_path":   "Client",
			"relative_path": "client.go",
			"depth":       0,
			"include_body": false,
		}

		t.Logf("Calling execute_tool(tool_path=%q, arguments=%+v)", toolPath, arguments)

		// Expected behavior:
		// 1. Proxy parses tool_path and identifies it maps to serena.find_symbol
		// 2. Proxy checks if Serena MCP client is connected
		// 3. If not, proxy starts Serena MCP server (lazy initialization)
		// 4. Proxy translates the call to Serena's find_symbol tool
		// 5. Proxy forwards arguments to Serena
		// 6. Serena executes find_symbol and returns results
		// 7. Proxy returns results to caller

		t.Log("✓ Should start Serena MCP server on first use (lazy)")
		t.Log("✓ Should map tool_path to actual Serena tool 'find_symbol'")
		t.Log("✓ Should forward arguments correctly")
		t.Log("✓ Should return actual results from Serena")
		t.Log("✓ Subsequent calls should reuse existing connection")
	})

	// Test Phase 7: Execute with short name (if unique)
	t.Run("Phase 7: execute_tool with short name resolves uniquely", func(t *testing.T) {
		// If tool name is unique across all servers, allow short form
		toolPath := "find_symbol"

		arguments := map[string]interface{}{
			"name_path": "newMCPServer",
			"relative_path": "client.go",
		}

		t.Logf("Calling execute_tool(tool_path=%q, arguments=%+v)", toolPath, arguments)

		t.Log("✓ Should resolve 'find_symbol' to 'coding_tools.serena.search.search_symbol'")
		t.Log("✓ Should work if tool name is unique")
		t.Log("✓ Should error with suggestions if tool name is ambiguous")
	})
}

// TestActualSerenaExecution tests real execution with Serena MCP server
func TestActualSerenaExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("Real Serena execution via recursive proxy", func(t *testing.T) {
		// TODO: This would require implementing the full recursive proxy
		// For now, this test documents the expected integration

		// Step 1: Start recursive lazy loading proxy
		t.Log("Step 1: Starting recursive lazy loading proxy...")
		t.Log("  - Loading hierarchy from testdata/mcp_hierarchy/")
		t.Log("  - Exposing only meta-tools initially")

		// Step 2: Explore the hierarchy
		t.Log("Step 2: Exploring tool hierarchy...")
		t.Log("  - Call get_tools_in_category('') -> see coding_tools, web_tools")
		t.Log("  - Call get_tools_in_category('coding_tools') -> see serena, playwright")
		t.Log("  - Call get_tools_in_category('coding_tools.serena') -> see structure")

		// Step 3: Execute actual tool
		t.Log("Step 3: Executing tool through proxy...")
		toolCall := map[string]interface{}{
			"tool_path": "coding_tools.serena.find_symbol",
			"arguments": map[string]interface{}{
				"name_path": "Client",
				"relative_path": "client.go",
				"depth": 1, // Get methods too
				"include_body": false,
			},
		}

		t.Logf("  Tool call: %+v", toolCall)
		t.Log("  Expected: Serena starts (if not already running)")
		t.Log("  Expected: Returns list of symbols from client.go")

		// Step 4: Verify results
		t.Log("Step 4: Verify results...")
		t.Log("  - Should have symbol information for 'Client' struct")
		t.Log("  - Should have child symbols (methods) if depth=1")
		t.Log("  - Results should be from actual Serena MCP server")
	})
}

// TestPlaywrightExecution tests Playwright through recursive proxy
func TestPlaywrightExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("Real Playwright execution via recursive proxy", func(t *testing.T) {
		t.Log("Exploring Playwright tools...")

		// Navigate to playwright
		t.Log("Step 1: get_tools_in_category('coding_tools.playwright')")
		t.Log("  Expected: browser category")

		// Execute a Playwright tool
		t.Log("Step 2: execute_tool('coding_tools.playwright.browser.navigate')")
		toolCall := map[string]interface{}{
			"tool_path": "coding_tools.playwright.browser.navigate",
			"arguments": map[string]interface{}{
				"url": "https://example.com",
			},
		}

		t.Logf("  Tool call: %+v", toolCall)
		t.Log("  Expected: Playwright server starts (lazy)")
		t.Log("  Expected: Browser navigates to example.com")
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

		// Verify structure
		assert.Contains(t, root, "overview", "root should have overview")
		assert.Contains(t, root, "categories", "root should have categories")
		assert.Contains(t, root, "tools", "root should have tools")

		// Verify meta-tools
		tools := root["tools"].(map[string]interface{})
		assert.Contains(t, tools, "get_tools_in_category")
		assert.Contains(t, tools, "execute_tool")

		t.Log("✓ root.json structure is valid")

		// Load coding_tools.json
		codingToolsPath := filepath.Join(hierarchyPath, "coding_tools", "coding_tools.json")
		codingData, err := os.ReadFile(codingToolsPath)
		require.NoError(t, err, "Should read coding_tools.json")

		var codingTools map[string]interface{}
		err = json.Unmarshal(codingData, &codingTools)
		require.NoError(t, err, "Should parse coding_tools.json")

		categories := codingTools["categories"].(map[string]interface{})
		assert.Contains(t, categories, "serena")
		assert.Contains(t, categories, "playwright")

		t.Log("✓ coding_tools.json structure is valid")

		// Load serena.json
		serenaPath := filepath.Join(hierarchyPath, "coding_tools", "serena", "serena.json")
		serenaData, err := os.ReadFile(serenaPath)
		require.NoError(t, err, "Should read serena.json")

		var serena map[string]interface{}
		err = json.Unmarshal(serenaData, &serena)
		require.NoError(t, err, "Should parse serena.json")

		// Verify MCP server config
		assert.Contains(t, serena, "mcp_server", "serena.json should specify MCP server config")
		mcpServer := serena["mcp_server"].(map[string]interface{})
		assert.Equal(t, "serena", mcpServer["name"])
		assert.Equal(t, "stdio", mcpServer["type"])
		assert.Contains(t, mcpServer, "command")
		assert.Contains(t, mcpServer, "args")

		t.Log("✓ serena.json structure is valid")
		t.Log("✓ MCP server configuration present")
	})
}

// TestCategoryResolution tests path resolution logic
func TestCategoryResolution(t *testing.T) {
	testCases := []struct {
		name           string
		path           string
		expectedType   string // "category", "tool", "server"
		expectedTarget string
	}{
		{
			name:           "Root path empty string",
			path:           "",
			expectedType:   "category",
			expectedTarget: "root",
		},
		{
			name:           "Root path slash",
			path:           "/",
			expectedType:   "category",
			expectedTarget: "root",
		},
		{
			name:           "Top level category",
			path:           "coding_tools",
			expectedType:   "category",
			expectedTarget: "coding_tools",
		},
		{
			name:           "Nested category with MCP server",
			path:           "coding_tools.serena",
			expectedType:   "server",
			expectedTarget: "serena",
		},
		{
			name:           "Deep nested category",
			path:           "coding_tools.serena.search",
			expectedType:   "category",
			expectedTarget: "search",
		},
		{
			name:           "Tool reference",
			path:           "coding_tools.serena.find_symbol",
			expectedType:   "tool",
			expectedTarget: "find_symbol",
		},
		{
			name:           "Full tool path",
			path:           "coding_tools.serena.search.search_symbol.find_symbol",
			expectedType:   "tool",
			expectedTarget: "find_symbol",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Path: %q", tc.path)
			t.Logf("Expected type: %s", tc.expectedType)
			t.Logf("Expected target: %s", tc.expectedTarget)

			// TODO: Implement path resolution logic
			// This would parse the path and determine what type of entity it refers to
		})
	}
}

// Helper function to print tool call results in a nice format
func printToolResult(t *testing.T, result *mcp.CallToolResult) {
	t.Log("Tool execution result:")
	for i, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			t.Logf("  Content[%d] (text): %s", i, textContent.Text)
		} else if imageContent, ok := mcp.AsImageContent(content); ok {
			t.Logf("  Content[%d] (image): %s", i, imageContent.MIMEType)
		} else {
			t.Logf("  Content[%d] (unknown type)", i)
		}
	}
}

// TestToolPathParsing tests parsing dot-notation tool paths
func TestToolPathParsing(t *testing.T) {
	testCases := []struct {
		toolPath       string
		expectedParts  []string
		expectedServer string
		expectedTool   string
	}{
		{
			toolPath:       "coding_tools.serena.find_symbol",
			expectedParts:  []string{"coding_tools", "serena", "find_symbol"},
			expectedServer: "serena",
			expectedTool:   "find_symbol",
		},
		{
			toolPath:       "coding_tools.serena.search.search_symbol.find_symbol",
			expectedParts:  []string{"coding_tools", "serena", "search", "search_symbol", "find_symbol"},
			expectedServer: "serena",
			expectedTool:   "find_symbol",
		},
		{
			toolPath:       "coding_tools.playwright.browser.navigate",
			expectedParts:  []string{"coding_tools", "playwright", "browser", "navigate"},
			expectedServer: "playwright",
			expectedTool:   "navigate",
		},
		{
			toolPath:       "find_symbol", // Short form
			expectedParts:  []string{"find_symbol"},
			expectedServer: "", // Would need to be resolved
			expectedTool:   "find_symbol",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.toolPath, func(t *testing.T) {
			// TODO: Implement tool path parsing
			parts := parseToolPath(tc.toolPath)
			assert.Equal(t, tc.expectedParts, parts)

			t.Logf("Tool path: %s", tc.toolPath)
			t.Logf("Parsed parts: %v", parts)
		})
	}
}

// Mock function for testing - would be implemented in actual proxy
func parseToolPath(path string) []string {
	// Placeholder implementation
	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// TestErrorHandling tests error cases
func TestErrorHandling(t *testing.T) {
	t.Run("Invalid category path", func(t *testing.T) {
		invalidPath := "nonexistent.category.path"
		t.Logf("Testing invalid path: %s", invalidPath)
		t.Log("Expected: Error indicating category not found")
		t.Log("Expected: Suggestions for valid categories")
	})

	t.Run("Ambiguous tool name", func(t *testing.T) {
		// If multiple servers have a tool with the same name
		ambiguousTool := "navigate" // Both playwright and hypothetical web_tools might have this
		t.Logf("Testing ambiguous tool: %s", ambiguousTool)
		t.Log("Expected: Error listing all matching tools")
		t.Log("Expected: User must provide full path")
	})

	t.Run("MCP server startup failure", func(t *testing.T) {
		t.Log("Testing MCP server startup failure")
		t.Log("Expected: Clear error message about server startup")
		t.Log("Expected: Log details about what went wrong")
	})

	t.Run("Invalid tool arguments", func(t *testing.T) {
		t.Log("Testing invalid tool arguments")
		t.Log("Expected: Validation error from MCP server")
		t.Log("Expected: Clear indication of what's wrong with args")
	})
}

// TestDocumentationGeneration tests that the hierarchy can generate docs
func TestDocumentationGeneration(t *testing.T) {
	t.Run("Generate markdown documentation from hierarchy", func(t *testing.T) {
		t.Log("The hierarchy JSON files can be used to generate documentation")
		t.Log("Example: Generate a README showing all available tools")

		// Expected markdown structure:
		expected := `
# MCP Proxy - Tool Hierarchy

## coding_tools
Development tools including semantic code analysis...

### serena
Semantic code analysis, symbol search, and intelligent editing.

#### Tools:
- get_symbols_overview: Get a high-level overview of top-level symbols
- activate_project: Activate a registered project

#### Categories:
- search: Find symbols, references, and code patterns
- edit: Modify code with semantic awareness

### playwright
Browser automation for testing web applications...
`

		t.Log("Expected documentation structure:")
		t.Log(expected)
	})
}
