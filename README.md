# Lazy MCP
Lazy MCP lets your agent fetch MCP tools only on demand, saving those tokens from polluting your context window.

In [this](https://voicetree.io/blog/lazy-mcp+for+tool+instructions+only+on+demand) example, it saved 17% (34,000 tokens) of an entire claude code context window by hiding 2 MCP tools that aren't always needed.

Welcoming open source contributions!

## How it Works

Lazy MCP exposes two meta tools, which allows agents to explore a tree structure of available MCP tools and categories.


- `get_tools_in_category(path)` - Navigate the tool hierarchy
- `execute_tool(tool_path, arguments)` - Execute tools by path


## Example Flow

```
1. get_tools_in_category("") → {
     "categories": {
       "coding_tools": "Development tools... use when...",
       "web_tools": "description ... instructions"
     }
   }
   
2. get_tools_in_category("coding_tools") → {
     "categories": {
       "serena": "description ... instructions",
     }
   } 

3. get_tools_in_category("coding_tools.serena") → {
     "tools": {"find_symbol": "...", "get_symbols_overview": "..."}
   }

4. execute_tool("coding_tools.serena.find_symbol", {...})
   → Lazy loads Serena server (if not already loaded)
   → Proxies request to Serena
   → Returns result
```

## Quick Start

```bash
make build
```

```bash
./build/structure_generator --config config.json --output testdata/mcp_hierarchy
```

This generates the hierarchical structure in the output folder config.json specifies, by fetching the available tools from the mcp servers specified.


**Add to Claude Code:**
```bash
 claude mcp add --transport stdio mcp-proxy build/mcp-proxy -- --config config.json
```

## Configuration

### Basic Config Structure

see [config.json](config.json) for an example.

### Tool Hierarchy Structure

Tool hierarchy is defined in `testdata/mcp_hierarchy/` with JSON files:

**Root node** (`testdata/mcp_hierarchy/root.json`):

**Category nodes** (e.g., `testdata/mcp_hierarchy/github/github.json`):

**Tool nodes** (e.g., `testdata/mcp_hierarchy/github/create_issue/create_issue.json`):

## Command Line Options

```bash
./mcp-proxy --help
```

## Credits

Forked from [TBXark/mcp-proxy](https://github.com/TBXark/mcp-proxy) - extended with hierarchical routing, lazy loading, and stdio support.

## License

MIT License - see [LICENSE](LICENSE)
