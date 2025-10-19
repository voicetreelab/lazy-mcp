# Configuration

## Basic Setup

```json
{
  "mcpProxy": {
    "baseURL": "http://localhost",
    "addr": ":8080",
    "name": "MCP Router",
    "version": "1.0.0",
    "type": "streamable-http",
    "options": {
      "logEnabled": true,
      "authTokens": []
    }
  },
  "mcpServers": {}
}
```

## Environment Variables

The config file supports environment variable expansion (enabled by default with `-expand-env`). Use `${VAR_NAME}` syntax:

```json
{
  "mcpServers": {
    "serena": {
      "command": "uv",
      "args": ["--directory", "${SERENA_PATH}", "run", "serena", "start-mcp-server"],
      "env": {}
    }
  }
}
```

Then set the environment variable:
```bash
export SERENA_PATH="/path/to/your/serena"
./build/mcp-proxy --config config.json
```

## mcpProxy

- `baseURL`: Public URL base for client endpoints
- `addr`: Bind address (e.g. `:8080`)
- `name`, `version`: Server identity for MCP handshake
- `type`: `sse` or `streamable-http`
- `options`:
  - `logEnabled` (bool): Enable request logging
  - `authTokens` ([]string): Valid bearer tokens for authentication

## Hierarchy Configuration

The router loads tool hierarchy from `testdata/mcp_hierarchy/` (default path). Each directory contains a JSON file defining:

**Root** (`root.json`):
```json
{
  "overview": "Description of what this level provides",
  "categories": {
    "coding_tools": "Development tools...",
    "web_tools": "Web scraping..."
  },
  "tools": {
    "get_tools_in_category": {
      "description": "Navigate the hierarchy",
      "inputSchema": {...}
    },
    "execute_tool": {
      "description": "Execute a tool by path",
      "inputSchema": {...}
    }
  }
}
```

**Category with MCP Server** (`coding_tools/serena/serena.json`):
```json
{
  "overview": "Serena semantic code analysis",
  "mcp_server": {
    "name": "serena",
    "type": "stdio",
    "command": "uv",
    "args": ["--directory", "/path/to/serena", "run", "serena", "start-mcp-server"],
    "env": {}
  },
  "categories": {
    "search": "Find symbols and references",
    "edit": "Modify code intelligently"
  },
  "tools": {
    "get_symbols_overview": {
      "description": "Get overview of file symbols",
      "maps_to": "get_symbols_overview"
    }
  }
}
```

### MCP Server Configuration

The `mcp_server` block supports:
- **stdio**: `command`, `args`, `env`
- **sse**: `url`, `headers`
- **streamable-http**: `url`, `headers`, `timeout`

Server configs are inherited by child categories (no need to repeat).

### Tool Mapping

- `maps_to`: Maps hierarchy tool name to actual MCP tool name
- If omitted, hierarchy name is used as-is
- Enables renaming tools for better organization

## Structure Example

```
testdata/mcp_hierarchy/
├── root.json
├── coding_tools/
│   ├── coding_tools.json
│   └── serena/
│       ├── serena.json          (MCP server config here)
│       ├── search/
│       │   └── search.json
│       └── edit/
│           └── edit.json
└── web_tools/
    └── web_tools.json
```

See example hierarchy in the repository.
