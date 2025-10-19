# MCP Router

Hierarchical MCP router that exposes 2 meta-tools for navigating and executing tools across multiple MCP servers.

## What It Does

Routes tool requests through a hierarchical structure, lazy-loading MCP servers only when their tools are executed. Instead of exposing 100+ tools upfront, it provides:
- `get_tools_in_category(path)` - Navigate the tool hierarchy
- `execute_tool(tool_path, arguments)` - Execute tools by path

The explorable structure is a tree, with two types of node:
 - category nodes
 - tool nodes

## Example Flow

```
1. List tools → ["get_tools_in_category", "execute_tool"]

2. get_tools_in_category("") → {
     "categories": {
       "coding_tools": "Development tools...",
       "web_tools": "Web scraping..."
     }
   }

3. get_tools_in_category("coding_tools.serena") → {
     "overview": "Semantic code analysis",
     "tools": {"find_symbol": "...", "get_symbols_overview": "..."}
   }

4. execute_tool("coding_tools.serena.find_symbol", {...})
   → Lazy loads Serena server (if not already loaded)
   → Proxies request to Serena
   → Returns result
```

## Benefits

Reduces LLM context by 95% - only 2 tools exposed initially instead of all tools from all servers, and servers load on-demand.

## Quick Start

### 1. Build the Binary


### 1. Generate Tool Hierarchy

If you want to customize the tool hierarchy:

```bash
make build
```

```bash
./build/structure_generator --config config.json --output testdata/mcp_hierarchy
```

This generates the hierarchical structure in `testdata/mcp_hierarchy/` based on your configured MCP servers.

Or use the Makefile:


**Add to Claude Code:**
```bash
 claude mcp add --transport stdio mcp-proxy build/mcp-proxy -- --config config.json
```

## Configuration

### Basic Config Structure

```json
{
  "mcpProxy": {
    "name": "MCP Proxy",
    "version": "1.0.0",
    "type": "stdio|sse|streamable-http",

    // HTTP-only settings (ignore for stdio)
    "baseURL": "http://localhost",
    "addr": ":9090",

    "options": {
      "lazyLoad": true,
      "logEnabled": true,
      "authTokens": ["your-token"]  // HTTP-only
    }
  },
  "mcpServers": {
    "server-name": {
      "transportType": "stdio|sse|streamable-http",

      // stdio client config
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-everything"],
      "env": {"VAR": "value"},

      // HTTP client config
      "url": "http://localhost:8080",
      "headers": {"Authorization": "Bearer token"},

      "options": {
        "lazyLoad": true
      }
    }
  }
}
```


### Tool Hierarchy Structure

Tool hierarchy is defined in `testdata/mcp_hierarchy/` with JSON files:

**Root node** (`testdata/mcp_hierarchy/root.json`):
```json
{
  "overview": "Root: 2 servers, 36 tools..."
}
```

**Category nodes** (e.g., `testdata/mcp_hierarchy/github/github.json`):
```json
{
  "overview": "github: 26 tools; create_issue -> Create issues..."
}
```

**Tool nodes** (e.g., `testdata/mcp_hierarchy/github/create_issue/create_issue.json`):
```json
{
  "tools": {
    "create_issue": {
      "description": "Create a new issue in a GitHub repository",
      "maps_to": "create_issue",
      "server": "github",
      "inputSchema": { ... }
    }
  }
}
```

## Command Line Options

```bash
./mcp-proxy --help
```

Options:
- `--config <path>` - Path to config file or HTTP(S) URL (default: `config.json`)
- `--insecure` - Allow insecure HTTPS when fetching config from URL
- `--expand-env` - Expand environment variables in config (default: `true`)
- `--http-headers <headers>` - Headers for config URL (format: `Key1:Value1;Key2:Value2`)
- `--http-timeout <seconds>` - Timeout for fetching config from URL (default: `10`)
- `--version` - Print version
- `--help` - Print help

## Docker

```bash
docker run -d -p 9090:9090 -v /path/to/config.json:/config/config.json ghcr.io/tbxark/mcp-proxy:latest
```

## Install Globally

```bash
go install github.com/TBXark/mcp-proxy@latest
mcp-proxy --config config.json
```

## Documentation

- Configuration: [docs/configuration.md](docs/CONFIGURATION.md)
- Usage: [docs/usage.md](docs/USAGE.md)
- Deployment: [docs/deployment.md](docs/DEPLOYMENT.md)

## Credits

Forked from [TBXark/mcp-proxy](https://github.com/TBXark/mcp-proxy) - extended with hierarchical routing, lazy loading, and stdio support.

## License

MIT License - see [LICENSE](LICENSE)
