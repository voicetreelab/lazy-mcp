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
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/voicetreelab/lazy-mcp/internal/client"
	"github.com/voicetreelab/lazy-mcp/internal/config"
	"github.com/voicetreelab/lazy-mcp/internal/metrics"
)

// HierarchyNode represents a node in the tool hierarchy
// Can be a branch node (has children) or leaf node (has tools)
type HierarchyNode struct {
	Overview  string                     `json:"overview,omitempty"`
	Tools     map[string]*ToolDefinition `json:"tools,omitempty"`
	MCPServer *MCPServerRef              `json:"mcp_server,omitempty"`
}

// ToolDefinition represents a tool in the hierarchy
type ToolDefinition struct {
	Description string                 `json:"description,omitempty"`
	MapsTo      string                 `json:"maps_to,omitempty"`
	Server      string                 `json:"server,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// HierarchyNodeData is used for unmarshaling JSON with flexible tool types
type HierarchyNodeData struct {
	Overview  string                 `json:"overview,omitempty"`
	Tools     map[string]interface{} `json:"tools,omitempty"`
	MCPServer *MCPServerRef          `json:"mcp_server,omitempty"`
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

		// Get filename without extension
		filename := strings.TrimSuffix(filepath.Base(path), ".json")

		// Get the directory name
		dirname := filepath.Base(filepath.Dir(path))

		// Determine hierarchy key based on structure
		var hierarchyKey string
		if filename == dirname {
			// Nested structure: directory/directory.json → use directory path only
			// e.g., everything/everything.json → "everything"
			hierarchyKey = strings.ReplaceAll(relPath, string(filepath.Separator), ".")
			if hierarchyKey == "." {
				hierarchyKey = ""
			}
		} else {
			// Flat structure: directory/tool.json → use directory.tool
			// e.g., everything/add.json → "everything.add"
			dirKey := strings.ReplaceAll(relPath, string(filepath.Separator), ".")
			if dirKey == "." || dirKey == "" {
				hierarchyKey = filename
			} else {
				hierarchyKey = dirKey + "." + filename
			}
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
		Overview:  nodeData.Overview,
		Tools:     make(map[string]*ToolDefinition),
		MCPServer: nodeData.MCPServer,
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
			if server, ok := toolMap["server"].(string); ok {
				tool.Server = server
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
// Returns a map with path, overview, children info, and tools
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

	// Find child nodes
	children := make(map[string]interface{})
	allChildrenAreLeaves := true
	aggregatedTools := make(map[string]interface{})

	for nodePath := range h.nodes {
		if nodePath == path || nodePath == "" {
			continue
		}

		// Check if this node is a direct child of the current path
		var isDirectChild bool
		var childName string

		if path == "" {
			// Root level - direct children have no dots
			if !strings.Contains(nodePath, ".") {
				isDirectChild = true
				childName = nodePath
			}
		} else {
			// Non-root - check if path is a prefix and child is one level deeper
			if strings.HasPrefix(nodePath, path+".") {
				remainder := strings.TrimPrefix(nodePath, path+".")
				if !strings.Contains(remainder, ".") {
					isDirectChild = true
					childName = remainder
				}
			}
		}

		if isDirectChild {
			childNode := h.nodes[nodePath]
			if len(childNode.Tools) > 0 {
				// Leaf node
				children[childName] = map[string]interface{}{
					"is_leaf":    true,
					"tool_count": len(childNode.Tools),
				}

				// Aggregate tools from leaf children
				for toolName, toolDef := range childNode.Tools {
					// In flat structure, nodePath already includes the tool name
					// e.g., "everything.echo" not "everything.echo.echo"
					toolPath := nodePath

					aggregatedTools[toolName] = map[string]interface{}{
						"description": toolDef.Description,
						"tool_path":   toolPath,
					}
				}
			} else {
				// Branch node
				allChildrenAreLeaves = false
				childInfo := map[string]interface{}{}
				if childNode.Overview != "" {
					childInfo["overview"] = childNode.Overview
				}
				children[childName] = childInfo
			}
		}
	}

	if len(children) > 0 {
		response["children"] = children
	}

	// If this node has direct tools or all children are leaves, include tools
	if len(node.Tools) > 0 {
		// Node has direct tools
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
	} else if allChildrenAreLeaves && len(aggregatedTools) > 0 {
		// All children are leaves - include their tools
		response["tools"] = aggregatedTools
	} else {
		response["tools"] = make(map[string]interface{})
	}

	return response, nil
}

// ResolveToolPath resolves a tool path to its definition and server name
// Returns the tool definition, server name (empty for meta-tools or if not configured), and any error
func (h *Hierarchy) ResolveToolPath(toolPath string) (*ToolDefinition, string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Parse the tool path
	parts := strings.Split(toolPath, ".")
	if len(parts) == 0 {
		return nil, "", fmt.Errorf("invalid tool path: %s", toolPath)
	}

	var foundTool *ToolDefinition

	// Strategy 1: Check if the full path is a node, and look for a tool with the same name as the last part
	// e.g., "everything.echo" -> check node "everything.echo" for tool "echo"
	lastPart := parts[len(parts)-1]
	if node, exists := h.nodes[toolPath]; exists {
		if tool, ok := node.Tools[lastPart]; ok {
			foundTool = tool
		}
	}

	// Strategy 2: Try to find the tool by progressively trying longer paths
	// e.g., for "coding_tools.serena.search.find_symbol":
	// - Try "coding_tools.serena.search" with tool "find_symbol"
	// - Then "coding_tools.serena" with tool "find_symbol"
	// - Then "coding_tools" with tool "find_symbol"
	// - Finally "" (root) with tool "find_symbol"
	if foundTool == nil {
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
					break
				}
			}
		}
	}

	if foundTool == nil {
		return nil, "", fmt.Errorf("tool not found: %s", toolPath)
	}

	// Return the tool and its server name (from the tool-level server field)
	return foundTool, foundTool.Server, nil
}

// HandleExecuteTool handles the execute_tool meta-tool
// Implements automatic retry with reconnection on transport errors (broken pipe, connection reset, etc.)
// Note: Timeout errors (context deadline exceeded) do NOT trigger retry to avoid duplicate operations.
func (h *Hierarchy) HandleExecuteTool(ctx context.Context, registry *ServerRegistry, toolPath string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Resolve the tool path to get tool definition and server name
	toolDef, serverName, err := h.ResolveToolPath(toolPath)
	if err != nil {
		return nil, err
	}

	if serverName == "" {
		return nil, fmt.Errorf("no MCP server configured for tool: %s", toolPath)
	}

	// Use the mapped tool name
	actualToolName := toolDef.MapsTo
	if actualToolName == "" {
		actualToolName = strings.Split(toolPath, ".")[len(strings.Split(toolPath, "."))-1]
	}

	// Retry config: max 2 attempts (initial + 1 retry) for transport errors only
	const maxAttempts = 2
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying tool execution after transport error (attempt %d/%d): %s", attempt+1, maxAttempts, toolPath)
		}

		// Get or load the MCP client for this server (inside loop for fresh connection on retry)
		mcpClient, err := registry.GetOrLoadServer(ctx, serverName)
		if err != nil {
			// GetOrLoadServer failure is not retryable (config issue, not transport)
			return nil, fmt.Errorf("failed to get MCP client: %w", err)
		}

		log.Printf("Executing tool: hierarchy_path=%s, server=%s, tool=%s (attempt %d/%d)", toolPath, serverName, actualToolName, attempt+1, maxAttempts)

		// Per-attempt timeout for tool execution (does not include retry time)
		// Note: We create the timeout BEFORE acquiring the lock to enforce a total deadline
		// for the operation. If we waited for the lock first, a client could hang indefinitely.
		toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		// Serialize tool calls to the same server to prevent concurrent stdio access.
		// Stdio is a single-channel transport that cannot handle interleaved messages.
		// See: https://github.com/voicetreelab/lazy-mcp/issues/8
		mutex := registry.GetClientMutex(serverName)
		mutex.Lock()

		// Track execution time for metrics
		startTime := time.Now()

		// Call the tool on the actual MCP server
		callRequest := mcp.CallToolRequest{}
		callRequest.Params.Name = actualToolName
		callRequest.Params.Arguments = arguments

		result, err := mcpClient.GetClient().CallTool(toolCtx, callRequest)
		duration := time.Since(startTime)
		cancel() // Clean up context
		mutex.Unlock()

		// Record metrics for this attempt
		if registry.metricsStore != nil {
			registry.metricsStore.RecordCall(serverName, toolPath, duration, err)
		}

		if err == nil {
			// Success - check if result has IsError set, append schema to help LLMs self-correct
			if result != nil && result.IsError && toolDef.InputSchema != nil && len(result.Content) > 0 {
				schemaJSON, marshalErr := json.MarshalIndent(toolDef.InputSchema, "", "  ")
				if marshalErr == nil {
					// Append schema to the first text content item
					// Note: TextContent is a value type, so we modify the copy and assign it back to the slice
					if textContent, ok := result.Content[0].(mcp.TextContent); ok {
						textContent.Text += fmt.Sprintf("\n\nExpected inputSchema:\n%s", string(schemaJSON))
						result.Content[0] = textContent
					}
				}
			}
			return result, nil
		}

		lastErr = err

		// Check if this is a transport error (broken pipe, connection reset, etc.)
		if isTransportError(err) {
			log.Printf("Transport error detected for server %s: %v", serverName, err)
			// Remove the stale client from cache so next attempt gets a fresh connection
			registry.RemoveClient(serverName)
			// Continue to retry
			continue
		}

		// Non-transport error, don't retry - include inputSchema in error message
		if toolDef.InputSchema != nil {
			schemaJSON, marshalErr := json.MarshalIndent(toolDef.InputSchema, "", "  ")
			if marshalErr == nil {
				return nil, fmt.Errorf("failed to call tool %s: %w\n\nExpected inputSchema:\n%s", actualToolName, err, string(schemaJSON))
			}
		}
		return nil, fmt.Errorf("failed to call tool %s: %w", actualToolName, err)
	}

	// All retries exhausted
	if toolDef.InputSchema != nil {
		schemaJSON, marshalErr := json.MarshalIndent(toolDef.InputSchema, "", "  ")
		if marshalErr == nil {
			return nil, fmt.Errorf("failed to call tool %s after retry: %w\n\nExpected inputSchema:\n%s", actualToolName, lastErr, string(schemaJSON))
		}
	}
	return nil, fmt.Errorf("failed to call tool %s after retry: %w", actualToolName, lastErr)
}

// ServerRegistry manages MCP client connections
type ServerRegistry struct {
	clients       map[string]*client.Client
	clientMutex   map[string]*sync.Mutex // Per-client mutex for serializing tool calls
	serverConfigs map[string]*config.MCPClientConfigV2
	metricsStore  *metrics.Store
	mu            sync.RWMutex
	// createClient optionally overrides client.NewMCPClient. Intended for tests
	// that inject mock transports; nil means use the production constructor.
	createClient func(name string, conf *config.MCPClientConfigV2) (*client.Client, error)
}

// NewServerRegistry creates a new server registry with server configurations
func NewServerRegistry(serverConfigs map[string]*config.MCPClientConfigV2) *ServerRegistry {
	return &ServerRegistry{
		clients:       make(map[string]*client.Client),
		clientMutex:   make(map[string]*sync.Mutex),
		serverConfigs: serverConfigs,
	}
}

// GetClientMutex returns a mutex for the given server, creating one if needed.
// This mutex serializes tool calls to prevent concurrent stdio access.
// Note: This map grows with the number of unique servers accessed. Since the set of
// servers is bounded by the configuration/hierarchy, this is not a memory leak.
func (r *ServerRegistry) GetClientMutex(serverName string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, exists := r.clientMutex[serverName]; exists {
		return m
	}

	m := &sync.Mutex{}
	r.clientMutex[serverName] = m
	return m
}

// SetMetricsStore sets the metrics store for recording tool call metrics
func (r *ServerRegistry) SetMetricsStore(store *metrics.Store) {
	r.metricsStore = store
}

// GetOrLoadServer gets an existing client or creates and initializes a new one
// This implements lazy loading - servers are only started when first accessed
func (r *ServerRegistry) GetOrLoadServer(ctx context.Context, serverName string) (*client.Client, error) {
	// First check with read lock
	r.mu.RLock()
	if client, exists := r.clients[serverName]; exists {
		r.mu.RUnlock()
		return client, nil
	}
	r.mu.RUnlock()

	// Need to create, use write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check again in case another goroutine created it
	if client, exists := r.clients[serverName]; exists {
		return client, nil
	}

	// Look up the server config
	cfg, exists := r.serverConfigs[serverName]
	if !exists {
		return nil, fmt.Errorf("server config not found: %s", serverName)
	}

	// Create the MCP client (tests may override via createClient)
	var mcpClient *client.Client
	var err error
	if r.createClient != nil {
		mcpClient, err = r.createClient(serverName, cfg)
	} else {
		mcpClient, err = client.NewMCPClient(serverName, cfg)
	}
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

	log.Printf("Created and initialized MCP client for server: %s", serverName)

	// Store the client
	r.clients[serverName] = mcpClient

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

	// Clear the clients and mutex maps
	r.clients = make(map[string]*client.Client)
	r.clientMutex = make(map[string]*sync.Mutex)
}

// RemoveClient removes a client from the registry, allowing it to be reconnected.
// This is used when a transport error is detected to force a fresh connection.
//
// Race condition note: If another goroutine is using this client while RemoveClient
// is called, the client will be closed mid-operation. This is safe because:
// 1. The operation will fail with "use of closed network connection"
// 2. This error is classified as a transport error, triggering a retry
// 3. The retry will get a fresh connection via GetOrLoadServer
func (r *ServerRegistry) RemoveClient(serverName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if client, exists := r.clients[serverName]; exists {
		log.Printf("Removing stale MCP client from cache: %s", serverName)
		_ = client.Close()
		delete(r.clients, serverName)
	}
}

// isTransportError checks if an error is a transport-level error indicating
// the connection is dead (broken pipe, connection reset, EOF, etc.)
// Note: "context deadline exceeded" is NOT considered a transport error because:
// 1. The server may still be processing the request (risk of duplicate operations)
// 2. Retrying a slow operation won't help if the server is overloaded
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Transport error patterns indicating dead connection
	// Note: "broken pipe" matches "write: broken pipe", "connection reset" matches "read: connection reset"
	transportErrors := []string{
		"broken pipe",
		"connection reset",
		"connection refused",
		"eof",
		"use of closed network connection",
		"no such host",
		"transport endpoint is not connected",
	}
	for _, pattern := range transportErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
