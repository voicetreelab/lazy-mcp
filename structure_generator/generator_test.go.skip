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
	// In the new structure, tools are in subdirectories, so server.json has categories, not tools
	assert.Equal(t, len(githubServer.Tools), len(githubNode.Categories), "GitHub category count should match tool count")
	assert.Empty(t, githubNode.Tools, "Server-level tools should be empty")

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
	// In the new structure, tools are in subdirectories, so server.json has categories, not tools
	assert.Equal(t, len(everythingServer.Tools), len(everythingNode.Categories), "Everything category count should match tool count")
	assert.Empty(t, everythingNode.Tools, "Server-level tools should be empty")

	// Verify structure
	t.Logf("\n=== Generated Structure ===")
	t.Logf("Root: %s", outputDir)
	t.Logf("  ├── root.json (%s)", rootNode.Overview)
	t.Logf("  ├── github/")
	t.Logf("  │   └── github.json (%d categories -> tools)", len(githubNode.Categories))
	t.Logf("  └── everything/")
	t.Logf("      └── everything.json (%d categories -> tools)", len(everythingNode.Categories))
	t.Logf("\nTotal: 2 servers, %d tools", len(githubNode.Categories)+len(everythingNode.Categories))
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
		assert.Empty(t, node.Categories, "Categories map should be empty")
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
		// In new structure, server-level has categories pointing to tool subdirectories
		assert.Len(t, node.Categories, 1, "Should have exactly 1 category")
		assert.Contains(t, node.Categories, "test_tool", "Should contain test_tool in categories")
		assert.Empty(t, node.Tools, "Server-level tools should be empty")

		// Verify the tool subdirectory exists
		toolDir := filepath.Join(outputDir, "single", "test_tool")
		_, err = os.Stat(toolDir)
		require.NoError(t, err, "Tool directory should exist")

		// Verify the tool JSON file
		toolJSON := filepath.Join(toolDir, "test_tool.json")
		toolData, err := os.ReadFile(toolJSON)
		require.NoError(t, err, "Should read test_tool.json")

		var toolNode ToolNode
		err = json.Unmarshal(toolData, &toolNode)
		require.NoError(t, err, "Should unmarshal test_tool.json")

		assert.Len(t, toolNode.Tools, 1, "Tool file should contain 1 tool")
		assert.Contains(t, toolNode.Tools, "test_tool", "Tool file should contain test_tool")
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

		// Should have tool2 from second server (in categories, not tools)
		assert.Contains(t, node.Categories, "tool2", "Should have tool from second server in categories")
	})
}

func TestRegenerateDirectory_NestedStructure(t *testing.T) {
	// This test simulates the drag-and-drop scenario:
	// 1. Generate initial structure
	// 2. Move some tools into a subdirectory (simulating drag-and-drop)
	// 3. Run regenerate
	// 4. Verify the parent's categories are updated

	// Setup: Create test server with tools
	servers := []ServerTools{{
		ServerName: "testserver",
		Tools: []Tool{
			{Name: "getThing1", Description: "Gets thing 1", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "getThing2", Description: "Gets thing 2", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "getThing3", Description: "Gets thing 3", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "doOther", Description: "Does other stuff", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}}

	outputDir := filepath.Join(t.TempDir(), "structure")

	// Step 1: Generate initial structure
	err := GenerateStructure(servers, outputDir)
	require.NoError(t, err, "Initial generation should succeed")

	// Verify initial state
	serverJSON := filepath.Join(outputDir, "testserver", "testserver.json")
	initialData, err := os.ReadFile(serverJSON)
	require.NoError(t, err)

	var initialNode ToolNode
	err = json.Unmarshal(initialData, &initialNode)
	require.NoError(t, err)
	assert.Len(t, initialNode.Categories, 4, "Should have 4 tools initially")

	// Step 2: Simulate drag-and-drop - create get_stuff subdirectory and move tools
	getStuffDir := filepath.Join(outputDir, "testserver", "get_stuff")
	err = os.MkdirAll(getStuffDir, 0755)
	require.NoError(t, err)

	// Move getThing1, getThing2, getThing3 into get_stuff/
	for _, toolName := range []string{"getThing1", "getThing2", "getThing3"} {
		srcDir := filepath.Join(outputDir, "testserver", toolName)
		dstDir := filepath.Join(getStuffDir, toolName)
		err = os.Rename(srcDir, dstDir)
		require.NoError(t, err, fmt.Sprintf("Should move %s", toolName))
	}

	// Step 3: Run regenerate
	err = RegenerateRootJSON(outputDir)
	require.NoError(t, err, "Regenerate should succeed")

	// Step 4: Verify results

	// Check get_stuff.json was created
	getStuffJSON := filepath.Join(getStuffDir, "get_stuff.json")
	_, err = os.Stat(getStuffJSON)
	require.NoError(t, err, "get_stuff.json should be created")

	// Read and verify get_stuff.json
	getStuffData, err := os.ReadFile(getStuffJSON)
	require.NoError(t, err)

	var getStuffNode ToolNode
	err = json.Unmarshal(getStuffData, &getStuffNode)
	require.NoError(t, err)

	assert.Len(t, getStuffNode.Categories, 3, "get_stuff should have 3 tools")
	assert.Contains(t, getStuffNode.Categories, "getThing1")
	assert.Contains(t, getStuffNode.Categories, "getThing2")
	assert.Contains(t, getStuffNode.Categories, "getThing3")
	assert.NotEmpty(t, getStuffNode.Overview, "get_stuff should have an overview")

	// Read and verify testserver.json was updated
	updatedData, err := os.ReadFile(serverJSON)
	require.NoError(t, err)

	var updatedNode ToolNode
	err = json.Unmarshal(updatedData, &updatedNode)
	require.NoError(t, err)

	// Should now have 2 categories: get_stuff and doOther
	assert.Len(t, updatedNode.Categories, 2, "testserver should now have 2 categories")
	assert.Contains(t, updatedNode.Categories, "get_stuff", "Should have get_stuff category")
	assert.Contains(t, updatedNode.Categories, "doOther", "Should still have doOther")
	assert.NotContains(t, updatedNode.Categories, "getThing1", "getThing1 should be removed from top level")
	assert.NotContains(t, updatedNode.Categories, "getThing2", "getThing2 should be removed from top level")
	assert.NotContains(t, updatedNode.Categories, "getThing3", "getThing3 should be removed from top level")

	// Verify the overview was pulled from get_stuff.json
	assert.Equal(t, getStuffNode.Overview, updatedNode.Categories["get_stuff"], "Parent should reference child's overview")

	t.Logf("\n=== After Reorganization ===")
	t.Logf("testserver.json categories: %v", updatedNode.Categories)
	t.Logf("get_stuff.json categories: %v", getStuffNode.Categories)
}

func TestRegenerateDirectory_DeeplyNested(t *testing.T) {
	// Test deeply nested structure: server/group1/group2/tool
	servers := []ServerTools{{
		ServerName: "testserver",
		Tools: []Tool{
			{Name: "tool1", Description: "Tool 1", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "tool2", Description: "Tool 2", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}}

	outputDir := filepath.Join(t.TempDir(), "structure")

	// Generate initial structure
	err := GenerateStructure(servers, outputDir)
	require.NoError(t, err)

	// Create nested structure: testserver/group1/group2/
	group1Dir := filepath.Join(outputDir, "testserver", "group1")
	group2Dir := filepath.Join(group1Dir, "group2")
	err = os.MkdirAll(group2Dir, 0755)
	require.NoError(t, err)

	// Move tool1 to group2
	srcDir := filepath.Join(outputDir, "testserver", "tool1")
	dstDir := filepath.Join(group2Dir, "tool1")
	err = os.Rename(srcDir, dstDir)
	require.NoError(t, err)

	// Move tool2 to group1
	srcDir = filepath.Join(outputDir, "testserver", "tool2")
	dstDir = filepath.Join(group1Dir, "tool2")
	err = os.Rename(srcDir, dstDir)
	require.NoError(t, err)

	// Run regenerate
	err = RegenerateRootJSON(outputDir)
	require.NoError(t, err)

	// Verify group2.json exists and has tool1
	group2JSON := filepath.Join(group2Dir, "group2.json")
	group2Data, err := os.ReadFile(group2JSON)
	require.NoError(t, err)

	var group2Node ToolNode
	err = json.Unmarshal(group2Data, &group2Node)
	require.NoError(t, err)
	assert.Contains(t, group2Node.Categories, "tool1")

	// Verify group1.json exists and has tool2 and group2
	group1JSON := filepath.Join(group1Dir, "group1.json")
	group1Data, err := os.ReadFile(group1JSON)
	require.NoError(t, err)

	var group1Node ToolNode
	err = json.Unmarshal(group1Data, &group1Node)
	require.NoError(t, err)
	assert.Contains(t, group1Node.Categories, "tool2")
	assert.Contains(t, group1Node.Categories, "group2")
	assert.Len(t, group1Node.Categories, 2)

	// Verify testserver.json has group1
	serverJSON := filepath.Join(outputDir, "testserver", "testserver.json")
	serverData, err := os.ReadFile(serverJSON)
	require.NoError(t, err)

	var serverNode ToolNode
	err = json.Unmarshal(serverData, &serverNode)
	require.NoError(t, err)
	assert.Contains(t, serverNode.Categories, "group1")
	assert.Len(t, serverNode.Categories, 1)

	t.Logf("\n=== Deeply Nested Structure ===")
	t.Logf("testserver -> group1 -> [tool2, group2]")
	t.Logf("testserver -> group1 -> group2 -> tool1")
}

func TestRegenerateDirectory_PreserveManualEdits(t *testing.T) {
	// Test that manual edits to overview are preserved during regeneration
	servers := []ServerTools{{
		ServerName: "testserver",
		Tools: []Tool{
			{Name: "tool1", Description: "Tool 1", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "tool2", Description: "Tool 2", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}}

	outputDir := filepath.Join(t.TempDir(), "structure")

	// Generate initial structure
	err := GenerateStructure(servers, outputDir)
	require.NoError(t, err)

	// Manually edit the server overview
	serverJSON := filepath.Join(outputDir, "testserver", "testserver.json")
	serverData, err := os.ReadFile(serverJSON)
	require.NoError(t, err)

	var serverNode ToolNode
	err = json.Unmarshal(serverData, &serverNode)
	require.NoError(t, err)

	customOverview := "My custom overview that should be preserved!"
	serverNode.Overview = customOverview

	// Write back the modified overview
	err = writeNodeToJSON(serverNode, serverJSON)
	require.NoError(t, err)

	// Create a subdirectory and move a tool
	subDir := filepath.Join(outputDir, "testserver", "subgroup")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	srcDir := filepath.Join(outputDir, "testserver", "tool1")
	dstDir := filepath.Join(subDir, "tool1")
	err = os.Rename(srcDir, dstDir)
	require.NoError(t, err)

	// Run regenerate
	err = RegenerateRootJSON(outputDir)
	require.NoError(t, err)

	// Verify the custom overview was preserved
	updatedData, err := os.ReadFile(serverJSON)
	require.NoError(t, err)

	var updatedNode ToolNode
	err = json.Unmarshal(updatedData, &updatedNode)
	require.NoError(t, err)

	assert.Equal(t, customOverview, updatedNode.Overview, "Manual overview edits should be preserved")
	assert.Contains(t, updatedNode.Categories, "subgroup", "Should have new subgroup")
	assert.Contains(t, updatedNode.Categories, "tool2", "Should still have tool2")
}

func TestCountTotalTools(t *testing.T) {
	// Create a test structure
	servers := []ServerTools{{
		ServerName: "testserver",
		Tools: []Tool{
			{Name: "tool1", Description: "Tool 1", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "tool2", Description: "Tool 2", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "tool3", Description: "Tool 3", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}}

	outputDir := filepath.Join(t.TempDir(), "structure")
	err := GenerateStructure(servers, outputDir)
	require.NoError(t, err)

	// Count tools in flat structure
	testServerPath := filepath.Join(outputDir, "testserver")
	count := countTotalTools(testServerPath)
	assert.Equal(t, 3, count, "Should count 3 tools in flat structure")

	// Create nested structure
	subDir := filepath.Join(outputDir, "testserver", "subgroup")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Move tool1 to subgroup
	srcDir := filepath.Join(outputDir, "testserver", "tool1")
	dstDir := filepath.Join(subDir, "tool1")
	err = os.Rename(srcDir, dstDir)
	require.NoError(t, err)

	// Regenerate to create subgroup.json
	err = RegenerateRootJSON(outputDir)
	require.NoError(t, err)

	// Count tools in nested structure
	count = countTotalTools(filepath.Join(outputDir, "testserver"))
	assert.Equal(t, 3, count, "Should still count 3 tools in nested structure")
}
