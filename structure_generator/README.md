# MCP Tool Structure Generator

Auto-generates a hierarchical folder structure from MCP server tool definitions.

## Quick Start

### Recommended: Using Pre-fetched Tool Data

```bash
# Generate structure from test data
go run cmd/main.go \
  -input tests/test_data/github_tools.json \
  -input tests/test_data/everything_tools.json \
  -output ./my_structure
```

### Advanced: Fetching from Live MCP Servers

**⚠️ Known Issue**: Direct stdio MCP server connections can hang during initialization. This is a limitation of the mcp-go client library with stdio servers.

**Recommended Workaround**: Fetch tools via the running MCP proxy's HTTP endpoint instead:

```bash
# 1. Start your MCP proxy
./mcp-proxy --config config.json

# 2. Fetch tools via HTTP and save to JSON
curl http://localhost:9090/list | jq '.tools' > my_server_tools.json

# 3. Use the fetched data
go run cmd/main.go -input my_server_tools.json
```

## CLI Usage

```bash
go run cmd/main.go [flags]

Flags:
  -input  string           Path to tool JSON file (can be repeated)
  -output string           Output directory (default: "./structure")
  -config string           Path to MCP server config (experimental, may hang)
  -regenerate-root bool    Regenerate root.json from existing structure

Examples:
  # Mode 1: Pre-fetched data (recommended)
  go run cmd/main.go \
    -input tests/test_data/github_tools.json \
    -input tests/test_data/everything_tools.json

  # Mode 2: Live servers (may hang on stdio servers)
  go run cmd/main.go -config tests/test_data/test_config.json

  # Mode 3: Regenerate structure after manual reorganization
  go run cmd/main.go -regenerate-root -output ./structure
```

## 🎨 Dynamic Tree Reorganization (Drag & Drop!)

The structure generator supports **effortless tree reorganization** - simply move folders around and regenerate. No code changes needed!

### Example Workflow

**1. Generate your initial structure from MCP servers:**

```bash
# Using a config file
go run cmd/main.go -config my_config.json -output ./my_tools
```

Example `my_config.json`:
```json
{
  "mcpServers": {
    "everything": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-everything"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "your-token"
      }
    }
  },
  "outputDir": "./my_tools"
}
```

**2. You get a flat structure:**

```
my_tools/
├── root.json
└── everything/
    ├── everything.json          # Lists all 11 tools
    ├── echo/
    │   └── echo.json
    ├── add/
    │   └── add.json
    ├── getResourceReference/
    │   └── getResourceReference.json
    ├── getResourceLinks/
    │   └── getResourceLinks.json
    ├── getTinyImage/
    │   └── getTinyImage.json
    └── ... (6 more tools)
```

**3. Reorganize by creating groups and moving folders:**

```bash
# Create a group for related tools
mkdir my_tools/everything/resources

# Move tools into the group (drag-and-drop or command line)
mv my_tools/everything/getResourceReference my_tools/everything/resources/
mv my_tools/everything/getResourceLinks my_tools/everything/resources/
mv my_tools/everything/getTinyImage my_tools/everything/resources/
```

**4. Regenerate with one command:**

```bash
go run cmd/main.go -regenerate-root -output ./my_tools
```

**5. Your structure is automatically updated! ✨**

```
my_tools/
├── root.json                    # Updated automatically
└── everything/
    ├── everything.json          # Now shows "resources" as a category
    ├── resources/               # New group created!
    │   ├── resources.json       # Auto-generated with 3 tools listed
    │   ├── getResourceReference/
    │   ├── getResourceLinks/
    │   └── getTinyImage/
    ├── echo/
    ├── add/
    └── ... (6 other tools)
```

**What happened:**
- ✅ `resources.json` was automatically created
- ✅ `everything.json` updated to include "resources" category
- ✅ The 3 moved tools were removed from `everything.json` top level
- ✅ Manual edits to overviews are preserved
- ✅ Tool counts updated recursively

### Key Benefits

🚀 **Zero Configuration** - Just move folders, then regenerate

🎯 **Infinite Nesting** - Create groups within groups as deep as you need

🔄 **Reversible** - Move tools back out and regenerate to flatten the structure

💾 **Preserves Edits** - Custom overview descriptions are never overwritten

⚡ **Fast Iteration** - Experiment with different organizations instantly

### Real-World Example

Let's say you have an "everything" server with mixed tools. You can organize them semantically:

```bash
# Before: 11 tools at the same level
everything.json → [echo, add, getResourceReference, getResourceLinks, getTinyImage, ...]

# After reorganization:
mkdir everything/resources
mkdir everything/math
mkdir everything/messaging

mv everything/getResource* everything/resources/
mv everything/getTinyImage everything/resources/
mv everything/add everything/math/
mv everything/echo everything/messaging/

# Run regenerate
go run cmd/main.go -regenerate-root -output ./my_tools

# Result: Clean hierarchy
everything/
├── everything.json → [resources, math, messaging, ...]
├── resources/
│   ├── resources.json → [getResourceReference, getResourceLinks, getTinyImage]
│   └── ... (tool folders)
├── math/
│   ├── math.json → [add]
│   └── add/
└── messaging/
    ├── messaging.json → [echo]
    └── echo/
```

## Generated Structure

### Hierarchical Structure (New!)

The generator now supports **unlimited nesting depth**. Organize your tools however makes sense for your use case:

```
structure/
├── root.json                              # Top-level overview
└── server_name/
    ├── server_name.json                   # Server overview with categories
    ├── group_name/                        # Your custom grouping
    │   ├── group_name.json                # Auto-generated group overview
    │   ├── tool1/
    │   │   └── tool1.json                 # Individual tool definition
    │   └── tool2/
    │       └── tool2.json
    └── standalone_tool/
        └── standalone_tool.json           # Tools can live at any level
```

### Previous: Two-Layer Hierarchy

Initially generates a flat two-layer structure:

```
structure/
├── root.json                    # Overview with server descriptions
├── github/
│   ├── github.json              # Server with 4 tool categories
│   ├── create_issue/
│   │   └── create_issue.json
│   └── ... (3 more tools)
└── everything/
    ├── everything.json          # Server with 11 tool categories
    ├── echo/
    │   └── echo.json
    └── ... (10 more tools)
```

### Root Overview Format

```json
{
  "overview": "MCP tool structure with 2 servers and 15 total tools. Available servers: github -> github MCP server with 4 tools, everything -> everything MCP server with 11 tools"
}
```

Each server description includes:
- Server name
- Tool count
- Auto-generated overview

### File Format

Each JSON file contains:
- `overview`: Description of the server/category/group
- `categories`: Map of subcategory names → their overview descriptions (for parent nodes)
- `tools`: Map of tool name → full MCP tool definition (for leaf nodes)

Example parent node `everything.json`:
```json
{
  "overview": "everything MCP server with 11 tools",
  "categories": {
    "resources": "resources containing 3 items",
    "echo": "Echoes back the input",
    "add": "Adds two numbers",
    ...
  },
  "tools": {}
}
```

Example leaf node `resources/getResourceReference/getResourceReference.json`:
```json
{
  "overview": "Returns a resource reference that can be used by MCP clients",
  "categories": {},
  "tools": {
    "getResourceReference": {
      "description": "Returns a resource reference that can be used by MCP clients",
      "maps_to": "getResourceReference",
      "inputSchema": {
        "type": "object",
        "properties": { ... }
      }
    }
  }
}
```

## Architecture

### Modules

1. **types.go** - MCP-compliant data structures (2025-06-18 spec)
2. **generator.go** - Two-layer structure generation with server descriptions
3. **cmd/main.go** - CLI with two modes: pre-fetched and live fetching

### Test Data

Real MCP tool data in `tests/test_data/`:
- `github_tools.json` - GitHub MCP server (4 tools)
- `everything_tools.json` - Official test server (11 tools)
- `test_config.json` - MCP server configuration

## Usage Examples

### Programmatic Usage

```go
import generator "github.com/voicetreelab/lazy-mcp/structure_generator"

// Load tool data
servers := []generator.ServerTools{
    {
        ServerName: "github",
        Tools: [...],
    },
}

// Generate structure
err := generator.GenerateStructure(servers, "./output")
```

## Troubleshooting

### MCP Server Connection Hangs

**Problem**: `go run cmd/main.go -config ...` hangs when connecting to stdio servers

**Root Cause**: The `mcp-go` client library has issues with stdio initialization for some servers

**Solutions**:
1. **Use pre-fetched data** (recommended):
   - Manually call the server's tools/list once
   - Save output to JSON
   - Use `-input` flag

2. **Fetch via running proxy**:
   ```bash
   # Start proxy first
   ./mcp-proxy --config config.json

   # Fetch via HTTP
   curl http://localhost:9090/list
   ```

3. **Use SSE/HTTP servers** (if supported):
   - Update config to use SSE or streamable-HTTP instead of stdio
   - These connection types work better with the client library

### Empty Structure Generated

**Problem**: Structure folder created but files are empty or missing

**Cause**: Server connection failed but error was logged as warning

**Fix**: Check logs for warnings like `⚠ Warning: Failed to fetch tools from...`

## Running Tests

```bash
cd structure_generator
go test -v
```

All tests use pre-fetched data and should pass reliably.

## Advanced Features

### Custom Overview Descriptions

You can manually edit any overview field in the JSON files. When you run `-regenerate-root`, your custom descriptions are preserved:

```bash
# 1. Edit a file
vim my_tools/everything/resources/resources.json
# Change overview to: "Resource management tools for MCP clients"

# 2. Regenerate
go run cmd/main.go -regenerate-root -output ./my_tools

# 3. Your custom overview is kept!
# And the parent everything.json will reference your custom description
```

### Moving Tools Between Servers

You can even move tools between different servers:

```bash
# Move a tool from "everything" to "github"
mv my_tools/everything/echo my_tools/github/

# Regenerate
go run cmd/main.go -regenerate-root -output ./my_tools

# Result: echo now appears in github.json categories
```

## Next Steps

- [x] ~~Implement tool grouping~~ **DONE!** Use `-regenerate-root`
- [x] ~~Support unlimited nesting depth~~ **DONE!**
- [ ] Add LLM-generated overviews for new groups
- [ ] Support fetching via MCP proxy HTTP endpoint
- [ ] Better error handling for stdio connection issues
