package hierarchy

import (
	"path/filepath"
	"testing"

	"github.com/TBXark/mcp-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadHierarchyBasic tests the basic hierarchy loading functionality
func TestLoadHierarchyBasic(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Verify root node was loaded
	rootNode, exists := h.nodes[""]
	require.True(t, exists, "root node should exist with empty string key")
	assert.NotEmpty(t, rootNode.Overview, "root should have overview")

	// Verify root is also accessible via "/" key
	rootNodeSlash, exists := h.nodes["/"]
	require.True(t, exists, "root node should exist with / key")
	assert.Equal(t, rootNode, rootNodeSlash, "/ and empty string should point to same root")

	// Verify meta-tools are loaded at root
	assert.Contains(t, rootNode.Tools, "get_tools_in_category")
	assert.Contains(t, rootNode.Tools, "execute_tool")
}

// TestLoadHierarchyStructure tests that the complete hierarchy structure is loaded
func TestLoadHierarchyStructure(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)

	// Test all expected nodes are loaded
	expectedNodes := []string{
		"",                                                // root
		"coding_tools",                                   // level 1
		"coding_tools.serena",                            // level 2 with server
		"coding_tools.serena.search",                     // level 3
		"coding_tools.serena.search.search_symbol",       // level 4
		"coding_tools.serena.edit",                       // level 3
		"coding_tools.playwright",                        // level 2 with server
	}

	for _, nodePath := range expectedNodes {
		t.Run("node_"+nodePath, func(t *testing.T) {
			node, exists := h.nodes[nodePath]
			assert.True(t, exists, "node %s should exist", nodePath)
			if exists {
				assert.NotEmpty(t, node.Overview, "node %s should have overview", nodePath)
			}
		})
	}
}

// TestLoadHierarchyStructureOverviews tests that overviews are loaded correctly
func TestLoadHierarchyStructureOverviews(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)

	// Check branch nodes have overviews
	rootNode := h.nodes[""]
	assert.NotEmpty(t, rootNode.Overview, "root should have overview")

	codingTools := h.nodes["coding_tools"]
	assert.NotEmpty(t, codingTools.Overview, "coding_tools should have overview")

	serena := h.nodes["coding_tools.serena"]
	assert.NotEmpty(t, serena.Overview, "serena should have overview")

	// Check leaf nodes don't have overviews but have tools
	edit := h.nodes["coding_tools.serena.edit"]
	assert.Empty(t, edit.Overview, "edit leaf node should not have overview")
	assert.NotEmpty(t, edit.Tools, "edit leaf node should have tools")
}

// TestLoadHierarchyTools tests that tools are loaded correctly
func TestLoadHierarchyTools(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)

	// Root meta-tools
	rootNode := h.nodes[""]
	assert.Len(t, rootNode.Tools, 2, "root should have exactly 2 meta-tools")

	getTool := rootNode.Tools["get_tools_in_category"]
	require.NotNil(t, getTool)
	assert.NotEmpty(t, getTool.Description)
	assert.NotNil(t, getTool.InputSchema)

	execTool := rootNode.Tools["execute_tool"]
	require.NotNil(t, execTool)
	assert.NotEmpty(t, execTool.Description)
	assert.NotNil(t, execTool.InputSchema)

	// Serena tools
	serena := h.nodes["coding_tools.serena"]
	assert.Contains(t, serena.Tools, "get_symbols_overview")
	assert.Contains(t, serena.Tools, "activate_project")

	symbolTool := serena.Tools["get_symbols_overview"]
	assert.NotEmpty(t, symbolTool.Description)
	assert.Equal(t, "get_symbols_overview", symbolTool.MapsTo)

	// Search symbol tool
	searchSymbol := h.nodes["coding_tools.serena.search.search_symbol"]
	assert.Contains(t, searchSymbol.Tools, "find_symbol")

	findSymbol := searchSymbol.Tools["find_symbol"]
	assert.NotEmpty(t, findSymbol.Description)
	assert.Equal(t, "find_symbol", findSymbol.MapsTo)
	assert.NotNil(t, findSymbol.InputSchema)

	// Edit tools
	edit := h.nodes["coding_tools.serena.edit"]
	assert.Contains(t, edit.Tools, "replace_symbol_body")
	assert.Contains(t, edit.Tools, "insert_after_symbol")
	assert.Contains(t, edit.Tools, "insert_before_symbol")
}

// TestLoadHierarchyServerConfigs tests that MCP server configs are loaded
func TestLoadHierarchyServerConfigs(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)

	// Root should not have server config
	rootNode := h.nodes[""]
	assert.Nil(t, rootNode.MCPServer, "root should not have server config")

	// coding_tools should not have server config
	codingTools := h.nodes["coding_tools"]
	assert.Nil(t, codingTools.MCPServer, "coding_tools should not have server config")

	// Serena should have server config
	serena := h.nodes["coding_tools.serena"]
	require.NotNil(t, serena.MCPServer, "serena should have server config")
	assert.Equal(t, "serena", serena.MCPServer.Name)
	assert.Equal(t, "stdio", serena.MCPServer.Type)
	assert.Equal(t, "serena", serena.MCPServer.Command)
	assert.NotEmpty(t, serena.MCPServer.Args)

	// Playwright should have server config
	playwright := h.nodes["coding_tools.playwright"]
	require.NotNil(t, playwright.MCPServer, "playwright should have server config")
	assert.Equal(t, "playwright", playwright.MCPServer.Name)
	assert.Equal(t, "stdio", playwright.MCPServer.Type)
	assert.Equal(t, "npx", playwright.MCPServer.Command)

	// Child nodes should not have server configs (inherited from parent)
	search := h.nodes["coding_tools.serena.search"]
	assert.Nil(t, search.MCPServer, "search should not have server config")

	edit := h.nodes["coding_tools.serena.edit"]
	assert.Nil(t, edit.MCPServer, "edit should not have server config")
}

// TestHandleGetToolsInCategory tests the meta-tool handler
func TestHandleGetToolsInCategory(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)

	tests := []struct {
		name            string
		path            string
		wantErr         bool
		checkResponse   func(t *testing.T, response map[string]interface{})
	}{
		{
			name:    "root with empty string",
			path:    "",
			wantErr: false,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "overview")
				assert.Contains(t, response, "children")
				assert.Contains(t, response, "tools")

				children := response["children"].(map[string]interface{})
				assert.Contains(t, children, "coding_tools")
			},
		},
		{
			name:    "root with slash",
			path:    "/",
			wantErr: false,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "children")
				children := response["children"].(map[string]interface{})
				assert.Contains(t, children, "coding_tools")
			},
		},
		{
			name:    "coding_tools",
			path:    "coding_tools",
			wantErr: false,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "overview")
				children := response["children"].(map[string]interface{})
				assert.Contains(t, children, "serena")
				assert.Contains(t, children, "playwright")
			},
		},
		{
			name:    "serena with tools and children",
			path:    "coding_tools.serena",
			wantErr: false,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "overview")
				assert.Contains(t, response, "tools")
				assert.Contains(t, response, "children")

				tools := response["tools"].(map[string]interface{})
				assert.Contains(t, tools, "get_symbols_overview")
				assert.Contains(t, tools, "activate_project")

				// Check tool has full path
				symbolTool := tools["get_symbols_overview"].(map[string]interface{})
				assert.Equal(t, "coding_tools.serena.get_symbols_overview", symbolTool["tool_path"])

				children := response["children"].(map[string]interface{})
				assert.Contains(t, children, "search")
				assert.Contains(t, children, "edit")
			},
		},
		{
			name:    "search category - parent of leaves",
			path:    "coding_tools.serena.search",
			wantErr: false,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				children := response["children"].(map[string]interface{})
				assert.Contains(t, children, "search_symbol")

				// Should include tools from leaf children
				tools := response["tools"].(map[string]interface{})
				assert.NotEmpty(t, tools, "parent of leaves should include aggregated tools")
			},
		},
		{
			name:    "invalid path",
			path:    "nonexistent",
			wantErr: true,
		},
		{
			name:    "invalid nested path",
			path:    "coding_tools.nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := h.HandleGetToolsInCategory(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, response)
			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestResolveToolPath tests tool path resolution
func TestResolveToolPath(t *testing.T) {
	hierarchyPath, err := filepath.Abs("../../testdata/mcp_hierarchy")
	require.NoError(t, err)

	h, err := LoadHierarchy(hierarchyPath)
	require.NoError(t, err)

	tests := []struct {
		name           string
		toolPath       string
		wantErr        bool
		wantMapsTo     string
		wantServerNil  bool
		checkServer    func(t *testing.T, cfg *config.MCPClientConfigV2)
	}{
		{
			name:          "root meta-tool get_tools_in_category",
			toolPath:      "get_tools_in_category",
			wantErr:       false,
			wantMapsTo:    "get_tools_in_category",
			wantServerNil: true, // Root tools don't have server
		},
		{
			name:          "root meta-tool execute_tool",
			toolPath:      "execute_tool",
			wantErr:       false,
			wantMapsTo:    "execute_tool",
			wantServerNil: true,
		},
		{
			name:          "serena direct tool",
			toolPath:      "coding_tools.serena.get_symbols_overview",
			wantErr:       false,
			wantMapsTo:    "get_symbols_overview",
			wantServerNil: false,
			checkServer: func(t *testing.T, cfg *config.MCPClientConfigV2) {
				assert.Equal(t, config.MCPClientTypeStdio, cfg.TransportType)
				assert.Equal(t, "serena", cfg.Command)
			},
		},
		{
			name:          "nested tool with server from parent",
			toolPath:      "coding_tools.serena.search.search_symbol.find_symbol",
			wantErr:       false,
			wantMapsTo:    "find_symbol",
			wantServerNil: false,
			checkServer: func(t *testing.T, cfg *config.MCPClientConfigV2) {
				assert.Equal(t, config.MCPClientTypeStdio, cfg.TransportType)
				assert.Equal(t, "serena", cfg.Command)
			},
		},
		{
			name:          "edit tool inherits server from serena",
			toolPath:      "coding_tools.serena.edit.replace_symbol_body",
			wantErr:       false,
			wantMapsTo:    "replace_symbol_body",
			wantServerNil: false,
			checkServer: func(t *testing.T, cfg *config.MCPClientConfigV2) {
				assert.Equal(t, "serena", cfg.Command)
			},
		},
		{
			name:     "nonexistent tool",
			toolPath: "coding_tools.nonexistent",
			wantErr:  true,
		},
		{
			name:     "invalid path",
			toolPath: "nonexistent.tool",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolDef, serverConfig, err := h.ResolveToolPath(tt.toolPath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, toolDef)
			assert.Equal(t, tt.wantMapsTo, toolDef.MapsTo)

			if tt.wantServerNil {
				assert.Nil(t, serverConfig, "expected no server config for %s", tt.toolPath)
			} else {
				require.NotNil(t, serverConfig, "expected server config for %s", tt.toolPath)
				if tt.checkServer != nil {
					tt.checkServer(t, serverConfig)
				}
			}
		})
	}
}

// TestMCPServerRefToClientConfig tests the conversion from MCPServerRef to MCPClientConfigV2
func TestMCPServerRefToClientConfig(t *testing.T) {
	tests := []struct {
		name   string
		ref    *MCPServerRef
		check  func(t *testing.T, cfg *config.MCPClientConfigV2)
	}{
		{
			name: "stdio server",
			ref: &MCPServerRef{
				Name:    "test-stdio",
				Type:    "stdio",
				Command: "test-cmd",
				Args:    []string{"arg1", "arg2"},
				Env:     map[string]string{"KEY": "value"},
			},
			check: func(t *testing.T, cfg *config.MCPClientConfigV2) {
				assert.Equal(t, config.MCPClientTypeStdio, cfg.TransportType)
				assert.Equal(t, "test-cmd", cfg.Command)
				assert.Equal(t, []string{"arg1", "arg2"}, cfg.Args)
				assert.Equal(t, map[string]string{"KEY": "value"}, cfg.Env)
			},
		},
		{
			name: "sse server",
			ref: &MCPServerRef{
				Name:    "test-sse",
				Type:    "sse",
				URL:     "http://localhost:8080",
				Headers: map[string]string{"Authorization": "Bearer token"},
			},
			check: func(t *testing.T, cfg *config.MCPClientConfigV2) {
				assert.Equal(t, config.MCPClientTypeSSE, cfg.TransportType)
				assert.Equal(t, "http://localhost:8080", cfg.URL)
				assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, cfg.Headers)
			},
		},
		{
			name: "streamable-http server",
			ref: &MCPServerRef{
				Name: "test-streamable",
				Type: "streamable-http",
				URL:  "http://localhost:9090",
			},
			check: func(t *testing.T, cfg *config.MCPClientConfigV2) {
				assert.Equal(t, config.MCPClientTypeStreamable, cfg.TransportType)
				assert.Equal(t, "http://localhost:9090", cfg.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.ref.ToClientConfig()
			require.NotNil(t, config)
			require.NotNil(t, config.Options)
			tt.check(t, config)
		})
	}
}

// TestLoadHierarchyErrorCases tests error handling
func TestLoadHierarchyErrorCases(t *testing.T) {
	t.Run("nonexistent path", func(t *testing.T) {
		_, err := LoadHierarchy("/nonexistent/path")
		assert.Error(t, err)
		// Error contains "failed to load root node" which includes the path to root.json
		assert.Contains(t, err.Error(), "root.json")
	})

	t.Run("invalid directory", func(t *testing.T) {
		// Create a temp file (not directory)
		tmpfile, err := filepath.Abs("testdata/invalid_file.txt")
		require.NoError(t, err)

		_, err = LoadHierarchy(tmpfile)
		assert.Error(t, err)
	})
}
