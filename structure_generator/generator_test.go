package structure_generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSimpleStructure(t *testing.T) {
	// Load test data - github and everything servers
	githubData, err := os.ReadFile(filepath.Join("tests", "test_data", "github_tools.json"))
	if err != nil {
		t.Fatalf("Failed to read github test data: %v", err)
	}

	everythingData, err := os.ReadFile(filepath.Join("tests", "test_data", "everything_tools.json"))
	if err != nil {
		t.Fatalf("Failed to read everything test data: %v", err)
	}

	var githubServer, everythingServer ServerTools
	if err := json.Unmarshal(githubData, &githubServer); err != nil {
		t.Fatalf("Failed to unmarshal github data: %v", err)
	}
	if err := json.Unmarshal(everythingData, &everythingServer); err != nil {
		t.Fatalf("Failed to unmarshal everything data: %v", err)
	}

	// Setup output directory
	outputDir := filepath.Join(t.TempDir(), "structure")

	// Generate structure with multiple servers
	err = GenerateStructure([]ServerTools{githubServer, everythingServer}, outputDir)
	if err != nil {
		t.Fatalf("GenerateStructure failed: %v", err)
	}

	// === Test Root Layer ===
	// Assert: structure/root.json exists
	rootJSON := filepath.Join(outputDir, "root.json")
	if _, err := os.Stat(rootJSON); os.IsNotExist(err) {
		t.Fatalf("Expected root.json to exist at %s", rootJSON)
	}

	// Read and verify root.json
	rootData, err := os.ReadFile(rootJSON)
	if err != nil {
		t.Fatalf("Failed to read root.json: %v", err)
	}

	var rootNode ToolNode
	if err := json.Unmarshal(rootData, &rootNode); err != nil {
		t.Fatalf("Failed to unmarshal root.json: %v", err)
	}

	if rootNode.Overview == "" {
		t.Error("Expected root overview to be non-empty")
	}

	// Verify overview contains server descriptions
	if !containsString(rootNode.Overview, "github -> github MCP server with 4 tools") {
		t.Errorf("Expected root overview to contain github server description, got: %s", rootNode.Overview)
	}
	if !containsString(rootNode.Overview, "everything -> everything MCP server with 11 tools") {
		t.Errorf("Expected root overview to contain everything server description, got: %s", rootNode.Overview)
	}
	if !containsString(rootNode.Overview, "2 servers and 15 total tools") {
		t.Errorf("Expected root overview to contain server and tool count, got: %s", rootNode.Overview)
	}

	t.Logf("Root overview: %s", rootNode.Overview)

	// === Test Server Layer - GitHub ===
	githubDir := filepath.Join(outputDir, "github")
	if _, err := os.Stat(githubDir); os.IsNotExist(err) {
		t.Fatalf("Expected github directory to exist at %s", githubDir)
	}

	githubJSON := filepath.Join(githubDir, "github.json")
	if _, err := os.Stat(githubJSON); os.IsNotExist(err) {
		t.Fatalf("Expected github.json to exist at %s", githubJSON)
	}

	githubData, err = os.ReadFile(githubJSON)
	if err != nil {
		t.Fatalf("Failed to read github.json: %v", err)
	}

	var githubNode ToolNode
	if err := json.Unmarshal(githubData, &githubNode); err != nil {
		t.Fatalf("Failed to unmarshal github.json: %v", err)
	}

	if githubNode.Overview == "" {
		t.Error("Expected github overview to be non-empty")
	}

	if len(githubNode.Tools) != 4 {
		t.Errorf("Expected 4 github tools, got %d", len(githubNode.Tools))
	}

	// === Test Server Layer - Everything ===
	everythingDir := filepath.Join(outputDir, "everything")
	if _, err := os.Stat(everythingDir); os.IsNotExist(err) {
		t.Fatalf("Expected everything directory to exist at %s", everythingDir)
	}

	everythingJSON := filepath.Join(everythingDir, "everything.json")
	if _, err := os.Stat(everythingJSON); os.IsNotExist(err) {
		t.Fatalf("Expected everything.json to exist at %s", everythingJSON)
	}

	everythingData, err = os.ReadFile(everythingJSON)
	if err != nil {
		t.Fatalf("Failed to read everything.json: %v", err)
	}

	var everythingNode ToolNode
	if err := json.Unmarshal(everythingData, &everythingNode); err != nil {
		t.Fatalf("Failed to unmarshal everything.json: %v", err)
	}

	if everythingNode.Overview == "" {
		t.Error("Expected everything overview to be non-empty")
	}

	if len(everythingNode.Tools) != 11 {
		t.Errorf("Expected 11 everything tools, got %d", len(everythingNode.Tools))
	}

	// Verify structure
	t.Logf("\n=== Generated Structure ===")
	t.Logf("Root: %s", outputDir)
	t.Logf("  ├── root.json (%s)", rootNode.Overview)
	t.Logf("  ├── github/")
	t.Logf("  │   └── github.json (%d tools)", len(githubNode.Tools))
	t.Logf("  └── everything/")
	t.Logf("      └── everything.json (%d tools)", len(everythingNode.Tools))
	t.Logf("\nTotal: 2 servers, %d tools", len(githubNode.Tools)+len(everythingNode.Tools))
}
