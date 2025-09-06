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

## Endpoints

Given `mcpProxy.baseURL = https://mcp.example.com` and a server key `fetch`:

- For `type: sse`: `https://mcp.example.com/fetch/sse`
- For `type: streamable-http`: `https://mcp.example.com/fetch/mcp`

## Auth

If `options.authTokens` is set for a server, requests must include a bearer token:

```
Authorization: <token>
```

If your client cannot set headers, embed the token in the route key (e.g. `fetch/<token>`) and call that path instead.

