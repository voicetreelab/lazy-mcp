package hierarchy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/TBXark/mcp-proxy/internal/client"
	"github.com/TBXark/mcp-proxy/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// HierarchyNode represents a node in the tool hierarchy
type HierarchyNode struct {
	Overview   string                    `json:"overview,omitempty"`
	Categories map[string]string         `json:"categories,omitempty"`
	Tools      map[string]*ToolDefinition `json:"tools,omitempty"`
	MCPServer  *MCPServerRef             `json:"mcp_server,omitempty"`
}

// ToolDefinition represents a tool in the hierarchy
type ToolDefinition struct {
	Description string                 `json:"description,omitempty"`
	MapsTo      string                 `json:"maps_to,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// HierarchyNodeData is used for unmarshaling JSON with flexible tool types
type HierarchyNodeData struct {
	Overview   string                 `json:"overview,omitempty"`
	Categories map[string]string      `json:"categories,omitempty"`
	Tools      map[string]interface{} `json:"tools,omitempty"`
	MCPServer  *MCPServerRef          `json:"mcp_server,omitempty"`
}

// MCPServerRef contains MCP server configuration
type MCPServerRef struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"` // "stdio", "sse", "streamable-http"
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	URL          string            `json:"url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ToolMappings map[string]string `json:"tool_mappings,omitempty"` // Maps hierarchy tool names to actual MCP tool names
}

// ToClientConfig converts MCPServerRef to MCPClientConfigV2
func (m *MCPServerRef) ToClientConfig() *config.MCPClientConfigV2 {
	cfg := &config.MCPClientConfigV2{
		Options: &config.OptionsV2{},
	}

	switch m.Type {
	case "stdio":
		cfg.TransportType = config.MCPClientTypeStdio
		cfg.Command = m.Command
		cfg.Args = m.Args
		cfg.Env = m.Env
	case "sse":
		cfg.TransportType = config.MCPClientTypeSSE
		cfg.URL = m.URL
		cfg.Headers = m.Headers
	case "streamable-http":
		cfg.TransportType = config.MCPClientTypeStreamable
		cfg.URL = m.URL
		cfg.Headers = m.Headers
	}

	return cfg
}

// Hierarchy manages the hierarchical tool structure
type Hierarchy struct {
	rootPath string
	nodes    map[string]*HierarchyNode
	mu       sync.RWMutex
}

// LoadHierarchy loads the hierarchy from a directory structure
func LoadHierarchy(hierarchyPath string) (*Hierarchy, error) {
	h := &Hierarchy{
		rootPath: hierarchyPath,
		nodes:    make(map[string]*HierarchyNode),
	}

	// Load root.json
	rootFile := filepath.Join(hierarchyPath, "root.json")
	rootNode, err := loadNode(rootFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load root node: %w", err)
	}
	h.nodes[""] = rootNode
	h.nodes["/"] = rootNode

	// Walk the directory structure and load all nodes
	err = filepath.Walk(hierarchyPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		if info.Name() == "root.json" {
			return nil // Already loaded
		}

		// Calculate the hierarchy path from the file path
		relPath, err := filepath.Rel(hierarchyPath, filepath.Dir(path))
		if err != nil {
			return err
		}

		// Convert directory path to dot notation
		hierarchyKey := strings.ReplaceAll(relPath, string(filepath.Separator), ".")
		if hierarchyKey == "." {
			hierarchyKey = ""
		}

		node, err := loadNode(path)
		if err != nil {
			log.Printf("Warning: failed to load node at %s: %v", path, err)
			return nil // Continue loading other nodes
		}

		h.nodes[hierarchyKey] = node
		log.Printf("Loaded hierarchy node: %s from %s", hierarchyKey, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk hierarchy: %w", err)
	}

	log.Printf("Loaded %d hierarchy nodes", len(h.nodes))
	return h, nil
}

// loadNode loads a single node from a JSON file
func loadNode(path string) (*HierarchyNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var nodeData HierarchyNodeData
	if err := json.Unmarshal(data, &nodeData); err != nil {
		return nil, err
	}

	// Convert to HierarchyNode with typed tools
	node := &HierarchyNode{
		Overview:   nodeData.Overview,
		Categories: nodeData.Categories,
		Tools:      make(map[string]*ToolDefinition),
		MCPServer:  nodeData.MCPServer,
	}

	// Parse tools - can be either map[string]interface{} or direct ToolDefinition
	for toolName, toolData := range nodeData.Tools {
		if toolMap, ok := toolData.(map[string]interface{}); ok {
			tool := &ToolDefinition{}
			if desc, ok := toolMap["description"].(string); ok {
				tool.Description = desc
			}
			if mapsTo, ok := toolMap["maps_to"].(string); ok {
				tool.MapsTo = mapsTo
			} else {
				// Default maps_to is the tool name itself
				tool.MapsTo = toolName
			}
			if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
				tool.InputSchema = schema
			}
			node.Tools[toolName] = tool
		}
	}

	return node, nil
}

// GetRootNode returns the root node of the hierarchy
func (h *Hierarchy) GetRootNode() *HierarchyNode {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.nodes[""]
}

// HandleGetToolsInCategory handles the get_tools_in_category meta-tool
// Returns a map with path, overview, categories, and tools
func (h *Hierarchy) HandleGetToolsInCategory(path string) (map[string]interface{}, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Normalize path
	if path == "/" {
		path = ""
	}
	path = strings.Trim(path, ".")

	// Find the node
	node, exists := h.nodes[path]
	if !exists {
		return nil, fmt.Errorf("category not found: %s", path)
	}

	// Build response
	response := map[string]interface{}{
		"path": path,
	}

	if node.Overview != "" {
		response["overview"] = node.Overview
	}

	if len(node.Categories) > 0 {
		response["categories"] = node.Categories
	}

	if len(node.Tools) > 0 {
		// Build tool info with full paths
		toolsInfo := make(map[string]interface{})
		for toolName, toolDef := range node.Tools {
			var toolPath string
			if path == "" {
				toolPath = toolName
			} else {
				toolPath = path + "." + toolName
			}

			toolsInfo[toolName] = map[string]interface{}{
				"description": toolDef.Description,
				"tool_path":   toolPath,
			}
		}
		response["tools"] = toolsInfo
	} else {
		response["tools"] = make(map[string]interface{})
	}

	return response, nil
}

// ResolveToolPath resolves a tool path to its definition and server config
// Returns the tool definition, server config (may be nil for meta-tools), and any error
func (h *Hierarchy) ResolveToolPath(toolPath string) (*ToolDefinition, *config.MCPClientConfigV2, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Parse the tool path
	parts := strings.Split(toolPath, ".")
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("invalid tool path: %s", toolPath)
	}

	var foundTool *ToolDefinition
	var serverConfig *config.MCPClientConfigV2
	var foundAtPath string

	// Try to find the tool by progressively trying longer paths
	// e.g., for "coding_tools.serena.search.find_symbol":
	// - Try "coding_tools.serena.search" with tool "find_symbol"
	// - Then "coding_tools.serena" with tool "find_symbol"
	// - Then "coding_tools" with tool "find_symbol"
	// - Finally "" (root) with tool "find_symbol"

	// Start from longest path and work backwards
	for i := len(parts) - 1; i >= 0; i-- {
		var categoryPath string
		var toolName string

		if i == 0 {
			// Single part or trying root
			categoryPath = ""
			toolName = parts[0]
		} else {
			categoryPath = strings.Join(parts[:i], ".")
			toolName = parts[len(parts)-1]
		}

		if node, exists := h.nodes[categoryPath]; exists {
			// Check if this node has the tool
			if tool, ok := node.Tools[toolName]; ok {
				foundTool = tool
				foundAtPath = categoryPath
				break
			}
		}
	}

	if foundTool == nil {
		return nil, nil, fmt.Errorf("tool not found: %s", toolPath)
	}

	// Find the server config by walking up from where we found the tool
	// First check the node where we found the tool
	if node, exists := h.nodes[foundAtPath]; exists && node.MCPServer != nil {
		serverConfig = node.MCPServer.ToClientConfig()
	} else {
		// Search parent paths for server config
		if foundAtPath != "" {
			parts := strings.Split(foundAtPath, ".")
			for i := len(parts); i >= 1; i-- {
				parentPath := strings.Join(parts[:i], ".")
				if node, exists := h.nodes[parentPath]; exists && node.MCPServer != nil {
					serverConfig = node.MCPServer.ToClientConfig()
					break
				}
			}
		}
	}

	return foundTool, serverConfig, nil
}

// HandleExecuteTool handles the execute_tool meta-tool
func (h *Hierarchy) HandleExecuteTool(ctx context.Context, registry *ServerRegistry, toolPath string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Resolve the tool path to get tool definition and server config
	toolDef, serverConfig, err := h.ResolveToolPath(toolPath)
	if err != nil {
		return nil, err
	}

	if serverConfig == nil {
		return nil, fmt.Errorf("no MCP server configured for tool: %s", toolPath)
	}

	// Get or load the MCP client for this server
	client, err := registry.GetOrLoadServer(ctx, serverConfig.Command, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP client: %w", err)
	}

	// Use the mapped tool name
	actualToolName := toolDef.MapsTo
	if actualToolName == "" {
		actualToolName = strings.Split(toolPath, ".")[len(strings.Split(toolPath, "."))-1]
	}

	log.Printf("Executing tool: hierarchy_path=%s, tool=%s", toolPath, actualToolName)

	// Call the tool on the actual MCP server
	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = actualToolName
	callRequest.Params.Arguments = arguments

	result, err := client.GetClient().CallTool(ctx, callRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", actualToolName, err)
	}

	return result, nil
}

// ServerRegistry manages MCP client connections
type ServerRegistry struct {
	clients map[string]*client.Client
	mu      sync.RWMutex
}

// NewServerRegistry creates a new server registry
func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{
		clients: make(map[string]*client.Client),
	}
}

// GetOrLoadServer gets an existing client or creates and initializes a new one
// This implements lazy loading - servers are only started when first accessed
func (r *ServerRegistry) GetOrLoadServer(ctx context.Context, name string, cfg *config.MCPClientConfigV2) (*client.Client, error) {
	// First check with read lock
	r.mu.RLock()
	if client, exists := r.clients[name]; exists {
		r.mu.RUnlock()
		return client, nil
	}
	r.mu.RUnlock()

	// Need to create, use write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check again in case another goroutine created it
	if client, exists := r.clients[name]; exists {
		return client, nil
	}

	// Create the MCP client
	mcpClient, err := client.NewMCPClient(name, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Start the client if needed
	if mcpClient.NeedManualStart() {
		err := mcpClient.GetClient().Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to start MCP client: %w", err)
		}
	}

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "mcp-proxy-recursive"}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = mcpClient.GetClient().Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	log.Printf("Created and initialized MCP client for server: %s", name)

	// Store the client
	r.clients[name] = mcpClient

	// Start ping task if needed
	if mcpClient.NeedPing() {
		go mcpClient.StartPingTask(ctx)
	}

	return mcpClient, nil
}

// Close closes all clients in the registry
func (r *ServerRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, client := range r.clients {
		log.Printf("Closing MCP client: %s", name)
		_ = client.Close()
	}
}
