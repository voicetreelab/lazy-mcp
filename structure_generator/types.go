package structure_generator

import "encoding/json"

// Tool represents an MCP tool definition per the 2025-06-18 spec
// https://modelcontextprotocol.io/specification/2025-06-18/server/tools
type Tool struct {
	Name        string                 `json:"name"`                  // Required: unique identifier
	Title       string                 `json:"title,omitempty"`       // Optional: human-readable display name
	Description string                 `json:"description,omitempty"` // Optional: what the tool does
	InputSchema map[string]interface{} `json:"inputSchema"`           // Required: JSON Schema for parameters
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"` // Optional: JSON Schema for output (2025-06-18)
	Annotations map[string]interface{} `json:"annotations,omitempty"`  // Optional: metadata for clients
}

// ServerTools represents all tools from a single MCP server
type ServerTools struct {
	ServerName string `json:"serverName"`
	Tools      []Tool `json:"tools"`
}

// ToolNode represents a node in the hierarchical tool structure
// Can be a branch node (has children) or leaf node (has tools)
type ToolNode struct {
	// Path is the fully qualified path (e.g., "coding_tools.serena.search")
	Path string `json:"path"`

	// Overview is a concise description of this node
	// Only present for branch nodes - leaf nodes use tool descriptions instead
	Overview string `json:"overview,omitempty"`

	// Tools maps tool names to their full definitions
	// Only present for leaf nodes
	Tools map[string]ToolDefinition `json:"tools,omitempty"`
}

// ToolDefinition is the detailed definition of a single tool for output
type ToolDefinition struct {
	Title        string                 `json:"title,omitempty"`
	Description  string                 `json:"description,omitempty"`
	MapsTo       string                 `json:"maps_to,omitempty"`        // Maps to actual MCP tool name
	Server       string                 `json:"server"`                   // The MCP server that provides this tool (required)
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"`
	Annotations  map[string]interface{} `json:"annotations,omitempty"`
}

// DomainCategory represents a top-level categorization
type DomainCategory string

const (
	CodingTools         DomainCategory = "coding_tools"
	WebTools            DomainCategory = "web_tools"
	DatabaseTools       DomainCategory = "database_tools"
	VersionControlTools DomainCategory = "version_control_tools"
	AITools             DomainCategory = "ai_tools"
	FileSystemTools     DomainCategory = "file_system_tools"
	Uncategorized       DomainCategory = "uncategorized"
)

// CategoryKeywords maps domain categories to identifying keywords
var CategoryKeywords = map[DomainCategory][]string{
	CodingTools: {
		"code", "edit", "search", "symbol", "refactor", "lint",
		"format", "ast", "syntax", "semantic", "ide", "lsp",
		"serena", "completion", "intellisense",
	},
	WebTools: {
		"http", "fetch", "request", "api", "rest", "graphql",
		"scrape", "crawl", "web", "url", "download", "upload",
	},
	DatabaseTools: {
		"database", "db", "sql", "query", "postgres", "mysql",
		"mongo", "redis", "cache", "store", "table", "collection",
	},
	VersionControlTools: {
		"git", "github", "gitlab", "commit", "branch", "merge",
		"pull", "push", "repository", "repo", "pr", "issue",
	},
	AITools: {
		"ai", "ml", "model", "embedding", "completion", "prompt",
		"llm", "gpt", "claude", "predict", "inference", "train",
	},
	FileSystemTools: {
		"file", "directory", "folder", "read", "write", "delete",
		"move", "copy", "rename", "list", "path", "fs",
	},
}

// ToolGroup represents a collection of related tools within a server
type ToolGroup struct {
	Name        string
	Description string
	Tools       []Tool
}

// CategorizedServer represents a server assigned to a domain category
type CategorizedServer struct {
	ServerName     string
	DomainCategory DomainCategory
	ToolGroups     []ToolGroup
	StandaloneTools []Tool // Tools that don't fit into any group
}

// GeneratorConfig configures the structure generator
type GeneratorConfig struct {
	// OutputDir is where the generated structure will be written
	OutputDir string

	// MinToolsForGroup is the minimum number of tools required to create a subcategory
	// If a group would have fewer tools, they're kept flat instead
	MinToolsForGroup int

	// UseSemanticClustering enables AI-based tool grouping (requires LLM)
	UseSemanticClustering bool

	// GenerateOverviews enables AI-generated overview descriptions
	GenerateOverviews bool
}

// DefaultGeneratorConfig returns sensible defaults
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		OutputDir:             "./generated_tools",
		MinToolsForGroup:      2,
		UseSemanticClustering: false,
		GenerateOverviews:     false,
	}
}

// MarshalJSON implements custom JSON marshaling for ToolNode
func (n *ToolNode) MarshalJSON() ([]byte, error) {
	// Create a map to omit path from JSON output
	output := map[string]interface{}{}

	// Only include overview if present (branch nodes)
	if n.Overview != "" {
		output["overview"] = n.Overview
	}

	// Only include tools if present (leaf nodes)
	if n.Tools != nil && len(n.Tools) > 0 {
		output["tools"] = n.Tools
	}

	// Return un-indented JSON - let the encoder handle indentation
	return json.Marshal(output)
}
