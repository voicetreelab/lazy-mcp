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

	// Process each server (skip root.json generation)
	for _, server := range servers {
		if err := generateServerStructure(server, outputDir); err != nil {
			return fmt.Errorf("failed to generate structure for server %s: %w", server.ServerName, err)
		}
	}

	// Generate root.json AFTER all server files are created
	if err := RegenerateRootJSON(outputDir); err != nil {
		return fmt.Errorf("failed to generate root.json: %w", err)
	}

	return nil
}

// RegenerateRootJSON regenerates root.json by reading existing server files
// This allows users to manually edit server overviews before regenerating the root
func RegenerateRootJSON(outputDir string) error {
	// First, recursively regenerate all subdirectories
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	// Regenerate each server directory recursively
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip files like root.json
		}

		serverDir := filepath.Join(outputDir, entry.Name())
		if err := RegenerateDirectory(serverDir, entry.Name()); err != nil {
			return fmt.Errorf("failed to regenerate directory %s: %w", entry.Name(), err)
		}
	}

	// Now generate root.json from the regenerated server files
	categories := make(map[string]string)
	var serverDescriptions []string
	totalTools := 0

	// Scan each server directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip files like root.json
		}

		serverName := entry.Name()
		serverJSONPath := filepath.Join(outputDir, serverName, serverName+".json")

		// Read the server's JSON file
		data, err := os.ReadFile(serverJSONPath)
		if err != nil {
			continue // Skip if file doesn't exist
		}

		var node ToolNode
		if err := json.Unmarshal(data, &node); err != nil {
			continue // Skip if invalid JSON
		}

		// Count tools from categories (since tools are in subdirectories)
		toolCount := countTotalTools(filepath.Join(outputDir, serverName))
		totalTools += toolCount

		// Add server to root's categories with its overview
		categories[serverName] = node.Overview

		// Use the overview from the file (respects manual edits)
		serverDescriptions = append(serverDescriptions, fmt.Sprintf("%s -> %s", serverName, node.Overview))
	}

	// Create overview text
	var overview string
	if len(serverDescriptions) == 0 {
		overview = "MCP tool structure with no servers"
	} else if len(serverDescriptions) == 1 {
		overview = fmt.Sprintf("MCP tool structure with 1 server and %d total tools. Available servers: %s",
			totalTools, serverDescriptions[0])
	} else {
		overview = fmt.Sprintf("MCP tool structure with %d servers and %d total tools. Available servers: %s",
			len(serverDescriptions), totalTools, joinServerDescriptions(serverDescriptions))
	}

	// Create root node with categories pointing to servers
	rootNode := ToolNode{
		Path:       "root",
		Overview:   overview,
		Categories: categories,
		Tools:      make(map[string]ToolDefinition),
	}

	// Write root.json
	rootPath := filepath.Join(outputDir, "root.json")
	return writeNodeToJSON(rootNode, rootPath)
}

// RegenerateDirectory recursively regenerates a directory's JSON file from its subdirectories
// This enables drag-and-drop reorganization: move tool folders around, then regenerate
func RegenerateDirectory(dirPath string, nodeName string) error {
	// Read all entries in this directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// First, recursively regenerate all subdirectories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDirPath := filepath.Join(dirPath, entry.Name())
		// Recursively regenerate subdirectory
		if err := RegenerateDirectory(subDirPath, entry.Name()); err != nil {
			return fmt.Errorf("failed to regenerate subdirectory %s: %w", entry.Name(), err)
		}
	}

	// Now collect categories from all subdirectories
	categories := make(map[string]string)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		childName := entry.Name()
		childJSONPath := filepath.Join(dirPath, childName, childName+".json")

		// Read the child's JSON file
		data, err := os.ReadFile(childJSONPath)
		if err != nil {
			continue // Skip if file doesn't exist
		}

		var childNode ToolNode
		if err := json.Unmarshal(data, &childNode); err != nil {
			continue // Skip if invalid JSON
		}

		// Add child to parent's categories with its overview
		categories[childName] = childNode.Overview
	}

	// Read existing JSON file to preserve any manual edits and check if it's a leaf node
	nodeJSONPath := filepath.Join(dirPath, nodeName+".json")
	existingOverview := ""
	isLeafNode := false

	existingData, err := os.ReadFile(nodeJSONPath)
	if err == nil {
		var existingNode ToolNode
		if json.Unmarshal(existingData, &existingNode) == nil {
			existingOverview = existingNode.Overview
			// If this node has tools, it's a leaf node - don't regenerate it
			if len(existingNode.Tools) > 0 {
				isLeafNode = true
			}
		}
	}

	// Don't regenerate leaf nodes (tool files) - they should be left as-is
	if isLeafNode {
		return nil
	}

	// Generate new overview if none exists or if we need to update it
	var overview string
	if existingOverview != "" {
		// Preserve manual edits
		overview = existingOverview
	} else {
		// Generate new overview
		if len(categories) == 0 {
			overview = fmt.Sprintf("%s with no items", nodeName)
		} else if len(categories) == 1 {
			overview = fmt.Sprintf("%s containing 1 item", nodeName)
		} else {
			overview = fmt.Sprintf("%s containing %d items", nodeName, len(categories))
		}
	}

	// Create the node
	node := ToolNode{
		Path:       nodeName,
		Overview:   overview,
		Categories: categories,
		Tools:      make(map[string]ToolDefinition),
	}

	// Write the updated JSON file
	return writeNodeToJSON(node, nodeJSONPath)
}

// countTotalTools recursively counts all tools in a directory tree
func countTotalTools(dirPath string) int {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0
	}

	total := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		childPath := filepath.Join(dirPath, entry.Name())
		childJSONPath := filepath.Join(childPath, entry.Name()+".json")

		data, err := os.ReadFile(childJSONPath)
		if err != nil {
			continue
		}

		var node ToolNode
		if err := json.Unmarshal(data, &node); err != nil {
			continue
		}

		// If this node has tools, count them (leaf node)
		if len(node.Tools) > 0 {
			total += len(node.Tools)
		} else if len(node.Categories) > 0 {
			// Otherwise, recursively count tools in subdirectories
			total += countTotalTools(childPath)
		}
	}

	return total
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
// New structure: server_name/server_name.json (parent) + server_name/tool_name/tool_name.json (children)
func generateServerStructure(server ServerTools, outputDir string) error {
	// Create server directory: structure/server_name/
	serverDir := filepath.Join(outputDir, server.ServerName)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	// Create individual tool files and collect their overviews for parent categories
	categories := make(map[string]string)
	for _, tool := range server.Tools {
		// Generate tool file
		if err := generateToolFile(tool, serverDir, server.ServerName); err != nil {
			return fmt.Errorf("failed to generate tool file for %s: %w", tool.Name, err)
		}

		// Add to parent's categories with tool's description as overview
		toolOverview := tool.Description
		if toolOverview == "" {
			toolOverview = fmt.Sprintf("Tool: %s", tool.Name)
		}
		categories[tool.Name] = toolOverview
	}

	// Create server-level ToolNode
	serverNode := ToolNode{
		Path:       server.ServerName,
		Overview:   generateServerOverview(server),
		Categories: categories, // Points to child tools
		Tools:      make(map[string]ToolDefinition), // Empty at server level
	}

	// Write server JSON file: structure/server_name/server_name.json
	jsonPath := filepath.Join(serverDir, server.ServerName+".json")
	return writeNodeToJSON(serverNode, jsonPath)
}

// generateToolFile creates a directory and JSON file for a single tool
// Structure: parent_dir/tool_name/tool_name.json
func generateToolFile(tool Tool, parentDir string, serverName string) error {
	// Create tool directory
	toolDir := filepath.Join(parentDir, tool.Name)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return fmt.Errorf("failed to create tool directory: %w", err)
	}

	// Create tool overview from description
	toolOverview := tool.Description
	if toolOverview == "" {
		toolOverview = fmt.Sprintf("%s tool from %s server", tool.Name, serverName)
	}

	// Create ToolNode for this tool
	toolNode := ToolNode{
		Path:       filepath.Join(serverName, tool.Name),
		Overview:   toolOverview,
		Categories: make(map[string]string), // Empty - tools are leaf nodes
		Tools: map[string]ToolDefinition{
			tool.Name: {
				Title:        tool.Title,
				Description:  tool.Description,
				MapsTo:       tool.Name, // Maps to the actual MCP tool name
				InputSchema:  tool.InputSchema,
				OutputSchema: tool.OutputSchema,
				Annotations:  tool.Annotations,
			},
		},
	}

	// Write tool JSON file
	jsonPath := filepath.Join(toolDir, tool.Name+".json")
	return writeNodeToJSON(toolNode, jsonPath)
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
	// Create file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Use encoder to avoid HTML escaping (like > becoming \u003e)
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	// Encode with pointer to invoke MarshalJSON method
	if err := encoder.Encode(&node); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
