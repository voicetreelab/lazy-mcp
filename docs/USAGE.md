# Usage

## CLI

```text
-config string         path to config file or a http(s) url (default "config.json")
-expand-env            expand environment variables in config file (default true)
-http-headers string   optional headers for config URL: 'Key1:Value1;Key2:Value2'
-http-timeout int      timeout (seconds) for remote config fetch (default 10)
-insecure              skip TLS verification for remote config
-version               print version and exit
-help                  print help and exit
```

## Meta-Tools

The router exposes 2 tools for navigating and executing tools across all MCP servers:

### `get_tools_in_category(path)`

Navigate the tool hierarchy and discover available tools.

**Arguments:**
- `path` (string): Category path using dot notation (e.g., `"coding_tools.serena"`) or `""` for root

**Returns:**
- `overview`: Description of the category
- `categories`: Available subcategories with descriptions
- `tools`: Available tools at this level with full paths

**Example:**
```json
get_tools_in_category("coding_tools.serena")
→ {
    "overview": "Serena semantic code analysis",
    "categories": {
      "search": "Find symbols and references",
      "edit": "Modify code intelligently"
    },
    "tools": {
      "get_symbols_overview": {
        "description": "Get overview of file symbols",
        "tool_path": "coding_tools.serena.get_symbols_overview"
      }
    }
  }
```

### `execute_tool(tool_path, arguments)`

Execute a tool by its full hierarchical path.

**Arguments:**
- `tool_path` (string): Full tool path (e.g., `"coding_tools.serena.find_symbol"`)
- `arguments` (object): Arguments to pass to the tool

**Behavior:**
- Lazy-loads the MCP server if not already running
- Proxies request to the actual MCP server
- Returns the tool's result

**Example:**
```json
execute_tool(
  "coding_tools.serena.find_symbol",
  {
    "name_path": "Client",
    "relative_path": "client.go",
    "depth": 1
  }
)
→ <result from Serena's find_symbol tool>
```

## Workflow

1. **List available tools**: `tools/list` → returns 2 meta-tools
2. **Explore root**: `get_tools_in_category("")` → see top-level categories
3. **Navigate deeper**: `get_tools_in_category("coding_tools")` → see dev tools
4. **Execute tool**: `execute_tool("coding_tools.serena.find_symbol", {...})` → runs the tool

## Auth

If `options.authTokens` is set, requests must include:

```
Authorization: Bearer <token>
```

## Endpoints

Given `mcpProxy.baseURL = http://localhost:8080`:

- For `type: sse`: `http://localhost:8080/sse`
- For `type: streamable-http`: `http://localhost:8080/mcp`
