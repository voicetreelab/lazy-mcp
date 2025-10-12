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
  -input  string   Path to tool JSON file (can be repeated)
  -output string   Output directory (default: "./structure")
  -config string   Path to MCP server config (experimental, may hang)

Examples:
  # Mode 1: Pre-fetched data (recommended)
  go run cmd/main.go \
    -input tests/test_data/github_tools.json \
    -input tests/test_data/everything_tools.json

  # Mode 2: Live servers (may hang on stdio servers)
  go run cmd/main.go -config tests/test_data/test_config.json
```

## Generated Structure

### Two-Layer Hierarchy

```
structure/
├── root.json                    # Overview with server descriptions
├── github/
│   └── github.json             # 4 tools with full schemas
└── everything/
    └── everything.json         # 11 tools with full schemas
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
- `overview`: Description of the server/category
- `tools`: Map of tool name → full MCP tool definition

Example `everything.json`:
```json
{
  "overview": "everything MCP server with 11 tools",
  "tools": {
    "echo": {
      "title": "Echo",
      "description": "Echoes back the input message",
      "inputSchema": { ... }
    },
    ...
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
import generator "github.com/TBXark/mcp-proxy/structure_generator"

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

## Next Steps

- [ ] Add categorization (coding_tools, web_tools, etc.)
- [ ] Implement tool grouping (search/, edit/, etc.)
- [ ] Add LLM-generated overviews
- [ ] Support fetching via MCP proxy HTTP endpoint
- [ ] Better error handling for stdio connection issues
