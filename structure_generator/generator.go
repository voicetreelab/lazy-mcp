package structure_generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateStructure creates a two-layer folder structure from MCP server tools
// Structure: structure/ (root) -> server_name/ (each server)
func GenerateStructure(servers []ServerTools, outputDir string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate root.json
	if err := generateRootJSON(servers, outputDir); err != nil {
		return fmt.Errorf("failed to generate root.json: %w", err)
	}

	// Process each server
	for _, server := range servers {
		if err := generateServerStructure(server, outputDir); err != nil {
			return fmt.Errorf("failed to generate structure for server %s: %w", server.ServerName, err)
		}
	}

	return nil
}

// generateRootJSON creates the root.json file with overview of all servers
func generateRootJSON(servers []ServerTools, outputDir string) error {
	// Build overview with server descriptions
	totalTools := 0
	var serverDescriptions []string

	for _, server := range servers {
		totalTools += len(server.Tools)
		serverOverview := generateServerOverview(server)
		serverDescriptions = append(serverDescriptions, fmt.Sprintf("%s -> %s", server.ServerName, serverOverview))
	}

	// Create overview text
	var overview string
	if len(servers) == 0 {
		overview = "MCP tool structure with no servers"
	} else if len(servers) == 1 {
		overview = fmt.Sprintf("MCP tool structure with 1 server and %d total tools. Available servers: %s",
			totalTools, serverDescriptions[0])
	} else {
		overview = fmt.Sprintf("MCP tool structure with %d servers and %d total tools. Available servers: %s",
			len(servers), totalTools, joinServerDescriptions(serverDescriptions))
	}

	// Create root node
	rootNode := ToolNode{
		Path:     "root",
		Overview: overview,
		Tools:    make(map[string]ToolDefinition),
	}

	// Write root.json
	rootPath := filepath.Join(outputDir, "root.json")
	return writeNodeToJSON(rootNode, rootPath)
}

// joinServerDescriptions joins server descriptions with proper formatting
func joinServerDescriptions(descriptions []string) string {
	if len(descriptions) == 0 {
		return ""
	}
	if len(descriptions) == 1 {
		return descriptions[0]
	}

	// Join all but last with ", " and last with " and "
	result := ""
	for i, desc := range descriptions {
		if i == 0 {
			result = desc
		} else if i == len(descriptions)-1 {
			result += ", " + desc
		} else {
			result += ", " + desc
		}
	}
	return result
}

// generateServerStructure creates the folder and JSON file for a single server
func generateServerStructure(server ServerTools, outputDir string) error {
	// Create server directory: structure/server_name/
	serverDir := filepath.Join(outputDir, server.ServerName)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	// Create ToolNode with name, overview, and extended tool data
	node := ToolNode{
		Path:     server.ServerName,
		Overview: generateServerOverview(server),
		Tools:    make(map[string]ToolDefinition),
	}

	// Convert tools to ToolDefinition map (extended data)
	for _, tool := range server.Tools {
		node.Tools[tool.Name] = ToolDefinition{
			Title:        tool.Title,
			Description:  tool.Description,
			InputSchema:  tool.InputSchema,
			OutputSchema: tool.OutputSchema,
			Annotations:  tool.Annotations,
		}
	}

	// Write JSON file: structure/server_name/server_name.json
	jsonPath := filepath.Join(serverDir, server.ServerName+".json")
	return writeNodeToJSON(node, jsonPath)
}

// generateServerOverview creates a simple overview for the server
func generateServerOverview(server ServerTools) string {
	toolCount := len(server.Tools)
	if toolCount == 0 {
		return fmt.Sprintf("%s MCP server with no tools", server.ServerName)
	}
	if toolCount == 1 {
		return fmt.Sprintf("%s MCP server with 1 tool", server.ServerName)
	}
	return fmt.Sprintf("%s MCP server with %d tools", server.ServerName, toolCount)
}

// writeNodeToJSON writes a ToolNode to a JSON file with pretty formatting
func writeNodeToJSON(node ToolNode, path string) error {
	// Use custom marshaling from ToolNode.MarshalJSON
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}
