# MCP Proxy Observability

The lazy-mcp proxy includes built-in observability tools that allow agents to self-diagnose performance issues, timeouts, and errors without requiring external monitoring infrastructure.

## Overview

Two MCP tools are provided:

| Tool | Description |
|------|-------------|
| `proxy_doctor` | Run health checks and get actionable suggestions (like `/doctor` command) |
| `get_proxy_metrics` | Get detailed latency percentiles, error rates, and recent errors |

## Quick Start

### Check Proxy Health

```
Agent: Let me check if there are any issues with the MCP proxy.
[calls proxy_doctor]

Response:
{
  "status": "warning",
  "checks": [
    {"name": "server_connectivity", "status": "pass", "message": "All 4 servers responding"},
    {"name": "latency", "status": "warning", "message": "google-calendar p99=45.1s"},
    {"name": "timeouts", "status": "fail", "message": "2 timeouts in last hour"},
    {"name": "error_rate", "status": "pass", "message": "Error rate 4.3%"},
    {"name": "memory", "status": "pass", "message": "Metrics store using ~0.2MB"}
  ],
  "suggestions": [
    "google-calendar: Consider using smaller parameters for list-events"
  ]
}
```

### Get Detailed Metrics

```
Agent: Show me the performance metrics for the last hour.
[calls get_proxy_metrics with since="1h"]

Response:
{
  "time_window": "1h",
  "summary": {
    "total_calls": 47,
    "success_rate": "95.7%",
    "timeout_count": 2,
    "active_servers": 4
  },
  "by_server": {
    "google-calendar": {
      "calls": 12,
      "success": 10,
      "timeouts": 2,
      "latency": {"p50": "2.3s", "p95": "15.2s", "p99": "45.1s"},
      "slowest_tool": "calendar.list-events",
      "last_error": "context deadline exceeded"
    },
    "gmail": {
      "calls": 20,
      "success": 20,
      "timeouts": 0,
      "latency": {"p50": "0.8s", "p95": "1.2s", "p99": "1.5s"}
    }
  },
  "recent_errors": [...]
}
```

## Configuration

Add these options to your proxy config under `mcpProxy.options`:

```json
{
  "mcpProxy": {
    "options": {
      "metricsEnabled": true,
      "metricsRetention": "8h",
      "metricsFile": ""
    }
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `metricsEnabled` | `true` | Enable/disable metrics collection |
| `metricsRetention` | `"8h"` | How long to keep metrics (e.g., "1h", "8h", "24h", "7d") |
| `metricsFile` | `""` | Path to persist metrics (empty = in-memory only) |

## Tool Reference

### proxy_doctor

Runs comprehensive health checks and returns actionable diagnostics.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {}
}
```

**Health Checks:**

| Check | Description | Thresholds |
|-------|-------------|------------|
| `server_connectivity` | Verifies all configured servers are connected | Pass if all responding |
| `latency` | Checks p99 latency per server | Warning: >30s, Critical: >45s |
| `timeouts` | Counts timeout events | Warning: ≥2, Critical: ≥4 |
| `error_rate` | Overall error percentage | Warning: >5%, Critical: >10% |
| `memory` | Metrics store memory usage | Informational |

### get_proxy_metrics

Returns detailed performance metrics with filtering options.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "server": {
      "type": "string",
      "description": "Filter by server name (optional)"
    },
    "since": {
      "type": "string",
      "description": "Time window: '1h', '24h', '7d' (default: '1h')"
    }
  }
}
```

## Architecture

### Data Flow

```
┌────────────────────────────────────────────────────────────────────┐
│                        lazy-mcp-proxy                               │
│                                                                     │
│  ┌─────────────┐         ┌─────────────────────────────────────┐   │
│  │ Tool Call   │         │          Metrics Store              │   │
│  │ execute_tool│────────▶│                                     │   │
│  │             │ record  │  ToolCall{                          │   │
│  └─────────────┘         │    Server: "gmail"                  │   │
│                          │    ToolPath: "gmail.search_emails"  │   │
│                          │    Duration: 234ms                  │   │
│                          │    Success: true                    │   │
│                          │  }                                  │   │
│                          │                                     │   │
│                          │  ServerStats{                       │   │
│                          │    Name: "gmail"                    │   │
│                          │    ColdStartMs: 812                 │   │
│                          │    IsConnected: true                │   │
│                          │  }                                  │   │
│                          └─────────────────────────────────────┘   │
│                                      │                              │
│                                      ▼                              │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │                       MCP Tools                                │ │
│  │                                                                │ │
│  │  proxy_doctor              get_proxy_metrics                   │ │
│  │  ├─ server_connectivity    ├─ latency percentiles (p50/95/99) │ │
│  │  ├─ latency check          ├─ success/error rates             │ │
│  │  ├─ timeout check          ├─ per-server breakdown            │ │
│  │  ├─ error_rate check       └─ recent errors list              │ │
│  │  └─ actionable suggestions                                     │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌────────────────────────────────────────────────────────────────────┐
│                         Agent (Claude)                              │
│                                                                     │
│  "Something seems slow" → proxy_doctor → "gmail p99=45s, suggest   │
│                                           using smaller queries"   │
└────────────────────────────────────────────────────────────────────┘
```

### Instrumentation Points

Metrics are collected at two key points:

1. **Tool Execution** (`hierarchy.go:HandleExecuteTool`)
   ```go
   startTime := time.Now()
   result, err := client.CallTool(ctx, request)
   metricsStore.RecordCall(server, tool, time.Since(startTime), err)
   ```

2. **Server Loading** (`hierarchy.go:GetOrLoadServer`)
   ```go
   startTime := time.Now()
   client := NewMCPClient(config)
   client.Initialize(ctx)
   metricsStore.RecordServerLoad(name, time.Since(startTime).Milliseconds())
   ```

### Why Zero Dependencies?

| Approach | Agent Can Self-Diagnose? | Setup Required |
|----------|-------------------------|----------------|
| **This (Zero-Dep)** | ✅ Yes - via MCP tools | None |
| Prometheus | ❌ No - needs human + Grafana | Significant |
| OpenTelemetry | ❌ No - needs collector + backend | Significant |

The key insight: **agents need to diagnose issues themselves**, not wait for human operators. This requires observability exposed via MCP tools.

## Design Principles

1. **Zero Dependencies** - No Prometheus, Grafana, or external services required
2. **Works Immediately** - Agents can query metrics out of the box
3. **Self-Diagnosing** - Agents understand issues and suggest fixes
4. **Lightweight** - In-memory with auto-pruning, <1MB memory footprint
5. **Optional Persistence** - Set `metricsFile` to survive restarts

## Tip: Enable Persistence

For debugging across proxy restarts, enable metrics persistence:

```json
{
  "mcpProxy": {
    "options": {
      "metricsFile": "~/.cache/lazy-mcp-metrics.json"
    }
  }
}
```

Without persistence, metrics are lost when the proxy restarts, making it harder to debug issues that occurred in previous sessions.
