# MCP Router

Hierarchical MCP router that exposes 2 meta-tools for navigating and executing tools across multiple MCP servers.

## What It Does

Routes tool requests through a hierarchical structure, lazy-loading MCP servers only when their tools are executed. Instead of exposing 100+ tools upfront, it provides:
- `get_tools_in_category(path)` - Navigate the tool hierarchy
- `execute_tool(tool_path, arguments)` - Execute tools by path

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

```bash
# Build
make build
./build/mcp-proxy --config config.json

# Or install
go install github.com/TBXark/mcp-proxy@latest

# Docker
docker run -d -p 9090:9090 -v /path/to/config.json:/config/config.json ghcr.io/tbxark/mcp-proxy:latest
```

## Configuration

```json
{
  "mcpProxy": {
    "baseURL": "http://localhost",
    "addr": ":8080",
    "name": "MCP Proxy",
    "version": "1.0.0",
    "type": "streamable-http"
  },
  "mcpServers": {}
}
```

Tool hierarchy configured in `testdata/mcp_hierarchy/` with JSON files defining categories, tools, and MCP server configs.

**Environment Variables**: Use `${VAR_NAME}` in config files for dynamic paths:
```bash
export SERENA_PATH="/path/to/your/serena"
./build/mcp-proxy --config config.lazy-load.json
```

See [docs/configuration.md](docs/CONFIGURATION.md) for details.

## Documentation

- Configuration: [docs/configuration.md](docs/CONFIGURATION.md)
- Usage: [docs/usage.md](docs/USAGE.md)
- Deployment: [docs/deployment.md](docs/DEPLOYMENT.md)

## Credits

Forked from [TBXark/mcp-proxy](https://github.com/TBXark/mcp-proxy) - extended with hierarchical routing and lazy loading.

## License

MIT License - see [LICENSE](LICENSE)
