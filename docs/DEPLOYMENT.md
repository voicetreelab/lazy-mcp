# Deployment

## Prerequisites

The router requires a hierarchy directory structure at `testdata/mcp_hierarchy/` (or custom path). Ensure this is available in your deployment environment.

## Docker

Run with config and hierarchy mounted:

```bash
docker run -d \
  -p 8080:8080 \
  -v /path/to/config.json:/config/config.json \
  -v /path/to/mcp_hierarchy:/app/testdata/mcp_hierarchy \
  ghcr.io/tbxark/mcp-proxy:latest
```

Or with remote config:

```bash
docker run -d -p 8080:8080 \
  -v /path/to/mcp_hierarchy:/app/testdata/mcp_hierarchy \
  ghcr.io/tbxark/mcp-proxy:latest \
  --config https://example.com/config.json
```

The image includes `npx` and `uvx` for launching MCP servers.

## Docker Compose

```yaml
services:
  mcp-router:
    image: ghcr.io/tbxark/mcp-proxy:latest
    pull_policy: always
    volumes:
      - ./config.json:/config/config.json
      - ./mcp_hierarchy:/app/testdata/mcp_hierarchy
    ports:
      - "8080:8080"
    restart: always
```

## Security

- Use `authTokens` for authentication
- Set `logEnabled: true` for debugging
- Ensure hierarchy JSON files are not writable at runtime
- MCP servers inherit security context from the router process

## Hierarchy Setup

Your deployment must include the hierarchy directory structure:

```
mcp_hierarchy/
├── root.json                 (defines meta-tools)
├── coding_tools/
│   ├── coding_tools.json
│   └── serena/
│       └── serena.json       (MCP server configs here)
└── web_tools/
    └── web_tools.json
```

See [CONFIGURATION.md](CONFIGURATION.md) for hierarchy format details.
