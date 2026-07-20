package hierarchy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	lazyclient "github.com/voicetreelab/lazy-mcp/internal/client"
	"github.com/voicetreelab/lazy-mcp/internal/config"
)

// reconnectMockTransport implements mcp-go's transport.Interface for tests.
//
// Coverage honesty:
//   - Exercises HandleExecuteTool's retry loop: isTransportError classification,
//     RemoveClient (evict + Close), GetOrLoadServer creating a fresh client, and
//     a successful second CallTool attempt.
//   - Uses a mock transport behind a real mark3labs/mcp-go Client (so CallTool /
//     Initialize / Close go through the real client path).
//   - Does NOT exercise real MCP stdio/SSE/HTTP transports, process spawning,
//     or OS-level broken-pipe from a dead subprocess.
type reconnectMockTransport struct {
	mu sync.Mutex

	// failNextToolsCalls is how many tools/call requests should fail before success.
	// Shared across generations when tests use a shared counter instead; per-instance
	// field is used when each client owns its own fail budget.
	failNextToolsCalls int
	failErr            error

	toolsCallCount atomic.Int32
	closed         atomic.Bool
	closeCount     atomic.Int32

	// optional shared counters for multi-client reconnect scenarios
	sharedToolsCalls *atomic.Int32
	// failWhileSharedBelow: fail tools/call while *sharedToolsCalls <= this value
	// (checked after increment). 0 means use per-instance failNextToolsCalls only.
	failWhileSharedBelow int32
}

func newReconnectMockTransport(failErr error, failNextToolsCalls int) *reconnectMockTransport {
	if failErr == nil {
		failErr = errors.New("write: broken pipe")
	}
	return &reconnectMockTransport{
		failErr:            failErr,
		failNextToolsCalls: failNextToolsCalls,
	}
}

func (t *reconnectMockTransport) Start(context.Context) error { return nil }

func (t *reconnectMockTransport) SendNotification(context.Context, mcp.JSONRPCNotification) error {
	return nil
}

func (t *reconnectMockTransport) SetNotificationHandler(func(mcp.JSONRPCNotification)) {}

func (t *reconnectMockTransport) GetSessionId() string { return "reconnect-mock" }

func (t *reconnectMockTransport) Close() error {
	t.closed.Store(true)
	t.closeCount.Add(1)
	return nil
}

func (t *reconnectMockTransport) SendRequest(ctx context.Context, request transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	switch request.Method {
	case "initialize":
		result, err := json.Marshal(map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"serverInfo": map[string]any{
				"name":    "reconnect-mock",
				"version": "test",
			},
		})
		if err != nil {
			return nil, err
		}
		return transport.NewJSONRPCResultResponse(request.ID, result), nil

	case "tools/call":
		n := t.toolsCallCount.Add(1)
		var sharedN int32
		if t.sharedToolsCalls != nil {
			sharedN = t.sharedToolsCalls.Add(1)
		}

		// Shared-counter mode (cross-client crash → reconnect)
		if t.sharedToolsCalls != nil && t.failWhileSharedBelow > 0 {
			if sharedN <= t.failWhileSharedBelow {
				return nil, t.failErr
			}
		} else if int(n) <= t.failNextToolsCalls {
			// Per-instance fail budget
			return nil, t.failErr
		}

		result, err := json.Marshal(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "recovered-ok"},
			},
		})
		if err != nil {
			return nil, err
		}
		return transport.NewJSONRPCResultResponse(request.ID, result), nil

	default:
		return nil, fmt.Errorf("reconnectMockTransport: unexpected method %q", request.Method)
	}
}

var _ transport.Interface = (*reconnectMockTransport)(nil)

// TestHandleExecuteTool_TransportCrashAutoReconnect is the highest-fidelity
// integration test the architecture allows without a real MCP subprocess.
//
// Flow under test:
//  1. First CallTool returns a transport error ("broken pipe")
//  2. isTransportError classifies it
//  3. RemoveClient evicts the stale client and Close()s it
//  4. Retry path GetOrLoadServer creates a fresh client (factory generation++)
//  5. Second CallTool succeeds
func TestHandleExecuteTool_TransportCrashAutoReconnect(t *testing.T) {
	const serverName = "crashy-server"
	const toolPath = "demo.echo"

	var (
		sharedToolsCalls atomic.Int32
		createCount      atomic.Int32
		closedTransports []*reconnectMockTransport
		closeMu          sync.Mutex
		clientsCreated   []*lazyclient.Client
	)

	makeClient := func(name string) (*lazyclient.Client, *reconnectMockTransport) {
		tr := &reconnectMockTransport{
			failErr:              errors.New("write: broken pipe"),
			sharedToolsCalls:     &sharedToolsCalls,
			failWhileSharedBelow: 1, // first tools/call across all clients fails
		}
		// Track Close via wrapper that records into closedTransports when closed
		// (transport itself sets closed; we append on create and assert after)
		closeMu.Lock()
		closedTransports = append(closedTransports, tr)
		closeMu.Unlock()

		mcpCli := mcpclient.NewClient(tr)
		wrapped := lazyclient.NewClientFromMCP(name, mcpCli)
		clientsCreated = append(clientsCreated, wrapped)
		return wrapped, tr
	}

	cfg := &config.MCPClientConfigV2{
		TransportType: config.MCPClientTypeStdio,
		Command:       "true", // unused; factory supplies clients
		Options:       &config.OptionsV2{},
	}
	registry := NewServerRegistry(map[string]*config.MCPClientConfigV2{
		serverName: cfg,
	})
	registry.createClient = func(name string, conf *config.MCPClientConfigV2) (*lazyclient.Client, error) {
		createCount.Add(1)
		c, _ := makeClient(name)
		return c, nil
	}

	h := &Hierarchy{
		nodes: map[string]*HierarchyNode{
			"demo.echo": {
				Tools: map[string]*ToolDefinition{
					"echo": {
						Description: "echo for reconnect test",
						MapsTo:      "echo",
						Server:      serverName,
					},
				},
			},
		},
	}

	// Sanity: the error we inject is classified as a transport error
	if !isTransportError(errors.New("write: broken pipe")) {
		t.Fatal("precondition: broken pipe should be a transport error")
	}

	ctx := context.Background()
	result, err := h.HandleExecuteTool(ctx, registry, toolPath, map[string]interface{}{
		"message": "hello",
	})
	if err != nil {
		t.Fatalf("HandleExecuteTool should recover after reconnect: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil CallToolResult after recovery")
	}
	if result.IsError {
		t.Fatalf("expected successful tool result, got IsError with content: %+v", result.Content)
	}

	// (a)+(d) two tools/call attempts: first crash, second success
	if got := sharedToolsCalls.Load(); got != 2 {
		t.Errorf("tools/call attempts = %d, want 2 (fail then succeed)", got)
	}

	// (c) two clients created: initial + reconnect
	if got := createCount.Load(); got != 2 {
		t.Errorf("client create count = %d, want 2 (initial + reconnect)", got)
	}
	if len(clientsCreated) != 2 {
		t.Fatalf("clientsCreated len = %d, want 2", len(clientsCreated))
	}
	if clientsCreated[0] == clientsCreated[1] {
		t.Error("reconnect should create a distinct client instance")
	}

	// (b) first client's transport was Closed via RemoveClient
	firstTr := closedTransports[0]
	if !firstTr.closed.Load() {
		t.Error("RemoveClient should Close() the stale client's transport")
	}
	if firstTr.closeCount.Load() < 1 {
		t.Error("expected Close() on stale transport")
	}

	// Registry holds the fresh (second) client
	if registry.clients[serverName] != clientsCreated[1] {
		t.Error("registry should hold the reconnected client")
	}

	// Text content from successful retry
	if len(result.Content) == 0 {
		t.Fatal("expected content in successful result")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", result.Content[0])
	}
	if text.Text != "recovered-ok" {
		t.Errorf("result text = %q, want %q", text.Text, "recovered-ok")
	}
}

// TestHandleExecuteTool_NonTransportErrorNoRetry verifies application errors
// do not trigger RemoveClient / reconnect.
func TestHandleExecuteTool_NonTransportErrorNoRetry(t *testing.T) {
	const serverName = "app-err-server"
	const toolPath = "demo.fail"

	var createCount atomic.Int32
	var toolsCalls atomic.Int32

	cfg := &config.MCPClientConfigV2{
		TransportType: config.MCPClientTypeStdio,
		Command:       "true",
		Options:       &config.OptionsV2{},
	}
	registry := NewServerRegistry(map[string]*config.MCPClientConfigV2{
		serverName: cfg,
	})
	registry.createClient = func(name string, conf *config.MCPClientConfigV2) (*lazyclient.Client, error) {
		createCount.Add(1)
		tr := &reconnectMockTransport{
			// Reuse reconnectMockTransport with a non-transport failErr and a large fail budget
			// (failNextToolsCalls: 99) so every tools/call returns a non-transport error → no retry.
			failErr:            errors.New("invalid arguments: missing field"),
			failNextToolsCalls: 99,
			sharedToolsCalls:   &toolsCalls,
			// failWhileSharedBelow 0 → per-instance budget of 99
		}
		// Force non-transport failure: failNextToolsCalls=99 means all calls fail
		// with failErr which is NOT a transport error → no retry.
		mcpCli := mcpclient.NewClient(tr)
		return lazyclient.NewClientFromMCP(name, mcpCli), nil
	}

	h := &Hierarchy{
		nodes: map[string]*HierarchyNode{
			"demo.fail": {
				Tools: map[string]*ToolDefinition{
					"fail": {
						Description: "always fails (app error)",
						MapsTo:      "fail",
						Server:      serverName,
					},
				},
			},
		},
	}

	_, err := h.HandleExecuteTool(context.Background(), registry, toolPath, nil)
	if err == nil {
		t.Fatal("expected error from non-transport failure")
	}
	if createCount.Load() != 1 {
		t.Errorf("create count = %d, want 1 (no reconnect)", createCount.Load())
	}
	// Client should still be in registry (not removed)
	if _, ok := registry.clients[serverName]; !ok {
		t.Error("client should remain in registry after non-transport error")
	}
}
