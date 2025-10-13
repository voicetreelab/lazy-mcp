package structure_generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSimpleStructure(t *testing.T) {
	// Load test data - github and everything servers
	githubData, err := os.ReadFile(filepath.Join("tests", "test_data", "github_tools.json"))
	require.NoError(t, err, "Failed to read github test data")

	everythingData, err := os.ReadFile(filepath.Join("tests", "test_data", "everything_tools.json"))
	require.NoError(t, err, "Failed to read everything test data")

	var githubServer, everythingServer ServerTools
	err = json.Unmarshal(githubData, &githubServer)
	require.NoError(t, err, "Failed to unmarshal github data")
	err = json.Unmarshal(everythingData, &everythingServer)
	require.NoError(t, err, "Failed to unmarshal everything data")

	// Setup output directory
	outputDir := filepath.Join(t.TempDir(), "structure")

	// Generate structure with multiple servers
	err = GenerateStructure([]ServerTools{githubServer, everythingServer}, outputDir)
	require.NoError(t, err, "GenerateStructure should succeed")

	// === Test Root Layer ===
	// Assert: structure/root.json exists
	rootJSON := filepath.Join(outputDir, "root.json")
	_, err = os.Stat(rootJSON)
	require.NoError(t, err, "root.json should exist")

	// Read and verify root.json
	rootData, err := os.ReadFile(rootJSON)
	require.NoError(t, err, "Should read root.json")

	var rootNode ToolNode
	err = json.Unmarshal(rootData, &rootNode)
	require.NoError(t, err, "Should unmarshal root.json")

	assert.NotEmpty(t, rootNode.Overview, "Root overview should not be empty")

	// Verify overview contains server descriptions (more flexible)
	totalTools := len(githubServer.Tools) + len(everythingServer.Tools)
	assert.Contains(t, rootNode.Overview, githubServer.ServerName, "Overview should mention github")
	assert.Contains(t, rootNode.Overview, everythingServer.ServerName, "Overview should mention everything")
	assert.Contains(t, rootNode.Overview, fmt.Sprintf("%d tools", len(githubServer.Tools)), "Overview should mention github tool count")
	assert.Contains(t, rootNode.Overview, fmt.Sprintf("%d tools", len(everythingServer.Tools)), "Overview should mention everything tool count")
	assert.Contains(t, rootNode.Overview, "2 servers", "Overview should mention server count")
	assert.Contains(t, rootNode.Overview, fmt.Sprintf("%d total tools", totalTools), "Overview should mention total tool count")

	t.Logf("Root overview: %s", rootNode.Overview)

	// === Test Server Layer - GitHub ===
	githubDir := filepath.Join(outputDir, "github")
	_, err = os.Stat(githubDir)
	require.NoError(t, err, "github directory should exist")

	githubJSON := filepath.Join(githubDir, "github.json")
	_, err = os.Stat(githubJSON)
	require.NoError(t, err, "github.json should exist")

	githubData, err = os.ReadFile(githubJSON)
	require.NoError(t, err, "Should read github.json")

	var githubNode ToolNode
	err = json.Unmarshal(githubData, &githubNode)
	require.NoError(t, err, "Should unmarshal github.json")

	assert.NotEmpty(t, githubNode.Overview, "GitHub overview should not be empty")
	assert.Equal(t, len(githubServer.Tools), len(githubNode.Tools), "GitHub tool count should match")

	// === Test Server Layer - Everything ===
	everythingDir := filepath.Join(outputDir, "everything")
	_, err = os.Stat(everythingDir)
	require.NoError(t, err, "everything directory should exist")

	everythingJSON := filepath.Join(everythingDir, "everything.json")
	_, err = os.Stat(everythingJSON)
	require.NoError(t, err, "everything.json should exist")

	everythingData, err = os.ReadFile(everythingJSON)
	require.NoError(t, err, "Should read everything.json")

	var everythingNode ToolNode
	err = json.Unmarshal(everythingData, &everythingNode)
	require.NoError(t, err, "Should unmarshal everything.json")

	assert.NotEmpty(t, everythingNode.Overview, "Everything overview should not be empty")
	assert.Equal(t, len(everythingServer.Tools), len(everythingNode.Tools), "Everything tool count should match")

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

func TestGenerateStructure_EdgeCases(t *testing.T) {
	t.Run("empty servers list", func(t *testing.T) {
		outputDir := filepath.Join(t.TempDir(), "structure")
		err := GenerateStructure([]ServerTools{}, outputDir)
		require.NoError(t, err, "Should succeed with empty servers list")

		// root.json should still be created
		rootJSON := filepath.Join(outputDir, "root.json")
		_, err = os.Stat(rootJSON)
		require.NoError(t, err, "root.json should exist")

		rootData, err := os.ReadFile(rootJSON)
		require.NoError(t, err, "Should read root.json")

		var rootNode ToolNode
		err = json.Unmarshal(rootData, &rootNode)
		require.NoError(t, err, "Should unmarshal root.json")

		assert.Contains(t, rootNode.Overview, "no servers", "Overview should mention no servers")
	})

	t.Run("server with no tools", func(t *testing.T) {
		servers := []ServerTools{{
			ServerName: "empty",
			Tools:      []Tool{},
		}}

		outputDir := filepath.Join(t.TempDir(), "structure")
		err := GenerateStructure(servers, outputDir)
		require.NoError(t, err, "Should succeed with server with no tools")

		// Check the server directory was created
		emptyDir := filepath.Join(outputDir, "empty")
		_, err = os.Stat(emptyDir)
		require.NoError(t, err, "empty directory should exist")

		// Check the server JSON file
		emptyJSON := filepath.Join(emptyDir, "empty.json")
		data, err := os.ReadFile(emptyJSON)
		require.NoError(t, err, "Should read empty.json")

		var node ToolNode
		err = json.Unmarshal(data, &node)
		require.NoError(t, err, "Should unmarshal empty.json")

		assert.Contains(t, node.Overview, "no tools", "Overview should mention no tools")
		assert.Empty(t, node.Tools, "Tools map should be empty")
	})

	t.Run("server with single tool", func(t *testing.T) {
		servers := []ServerTools{{
			ServerName: "single",
			Tools: []Tool{{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{"type": "object"},
			}},
		}}

		outputDir := filepath.Join(t.TempDir(), "structure")
		err := GenerateStructure(servers, outputDir)
		require.NoError(t, err, "Should succeed with single tool")

		// Verify the tool was written correctly
		singleJSON := filepath.Join(outputDir, "single", "single.json")
		data, err := os.ReadFile(singleJSON)
		require.NoError(t, err, "Should read single.json")

		var node ToolNode
		err = json.Unmarshal(data, &node)
		require.NoError(t, err, "Should unmarshal single.json")

		assert.Contains(t, node.Overview, "1 tool", "Overview should mention 1 tool")
		assert.Len(t, node.Tools, 1, "Should have exactly 1 tool")
		assert.Contains(t, node.Tools, "test_tool", "Should contain test_tool")
	})

	t.Run("invalid output directory", func(t *testing.T) {
		// Try to write to a path that can't be created (file as parent)
		tempFile := filepath.Join(t.TempDir(), "file.txt")
		err := os.WriteFile(tempFile, []byte("test"), 0644)
		require.NoError(t, err)

		invalidPath := filepath.Join(tempFile, "subdir", "structure")
		servers := []ServerTools{{ServerName: "test", Tools: []Tool{}}}

		err = GenerateStructure(servers, invalidPath)
		assert.Error(t, err, "Should fail with invalid output directory")
	})

	t.Run("duplicate server names", func(t *testing.T) {
		servers := []ServerTools{
			{ServerName: "duplicate", Tools: []Tool{{Name: "tool1", InputSchema: map[string]interface{}{}}}},
			{ServerName: "duplicate", Tools: []Tool{{Name: "tool2", InputSchema: map[string]interface{}{}}}},
		}

		outputDir := filepath.Join(t.TempDir(), "structure")
		err := GenerateStructure(servers, outputDir)
		require.NoError(t, err, "Should succeed (second overwrites first)")

		// The second server should have overwritten the first
		dupJSON := filepath.Join(outputDir, "duplicate", "duplicate.json")
		data, err := os.ReadFile(dupJSON)
		require.NoError(t, err)

		var node ToolNode
		err = json.Unmarshal(data, &node)
		require.NoError(t, err)

		// Should have tool2 from second server
		assert.Contains(t, node.Tools, "tool2", "Should have tool from second server")
	})
}
