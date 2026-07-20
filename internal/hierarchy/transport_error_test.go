package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	lazyclient "github.com/voicetreelab/lazy-mcp/internal/client"
	"github.com/voicetreelab/lazy-mcp/internal/config"
)

func TestIsTransportError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// Should detect as transport errors
		{
			name:     "broken pipe",
			err:      errors.New("write: broken pipe"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("read: connection reset by peer"),
			expected: true,
		},
		{
			name:     "EOF",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "closed network connection",
			err:      errors.New("use of closed network connection"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "transport endpoint not connected",
			err:      errors.New("transport endpoint is not connected"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      errors.New("dial tcp: lookup foo: no such host"),
			expected: true,
		},
		// Case insensitivity
		{
			name:     "BROKEN PIPE uppercase",
			err:      errors.New("BROKEN PIPE"),
			expected: true,
		},
		// Wrapped errors (%w) — isTransportError uses err.Error() substring match,
		// so wrapping preserves detectability via the error string.
		{
			name:     "wrapped connection reset via %w",
			err:      fmt.Errorf("context: %w", errors.New("connection reset")),
			expected: true,
		},
		{
			name:     "wrapped broken pipe via %w",
			err:      fmt.Errorf("call tool: %w", errors.New("write: broken pipe")),
			expected: true,
		},
		{
			name:     "double-wrapped EOF via %w",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", errors.New("unexpected EOF"))),
			expected: true,
		},
		{
			name:     "mcp-go transport.Error wrapper style",
			err:      fmt.Errorf("transport error: %v", errors.New("connection reset by peer")),
			expected: true,
		},
		// Should NOT detect as transport errors
		// IMPORTANT: Timeout errors are NOT transport errors because:
		// 1. The server may still be processing (risk of duplicate operations)
		// 2. Retrying a slow operation won't help if server is overloaded
		{
			name:     "context deadline exceeded - NOT a transport error",
			err:      errors.New("context deadline exceeded"),
			expected: false, // Changed from true - retrying could cause duplicates!
		},
		{
			name:     "i/o timeout - NOT a transport error",
			err:      errors.New("i/o timeout"),
			expected: false, // Changed from true - server may still be working
		},
		{
			name:     "connection timed out - NOT a transport error",
			err:      errors.New("dial tcp: connection timed out"),
			expected: false, // Changed from true - could be temporary network issue
		},
		{
			name:     "wrapped non-transport error via %w",
			err:      fmt.Errorf("context: %w", errors.New("invalid arguments")),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "tool not found",
			err:      errors.New("tool not found: foo.bar"),
			expected: false,
		},
		{
			name:     "invalid arguments",
			err:      errors.New("invalid arguments: missing required field"),
			expected: false,
		},
		{
			name:     "server config not found",
			err:      errors.New("server config not found: playwright"),
			expected: false,
		},
		{
			name:     "permission denied",
			err:      errors.New("permission denied"),
			expected: false,
		},
		{
			name:     "file not found",
			err:      errors.New("open /path/to/file: no such file or directory"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTransportError(tt.err)
			if result != tt.expected {
				t.Errorf("isTransportError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// closeTrackingTransport is a minimal mcp-go transport that records Close().
type closeTrackingTransport struct {
	closed     atomic.Bool
	closeCount atomic.Int32
}

func (t *closeTrackingTransport) Start(context.Context) error { return nil }
func (t *closeTrackingTransport) SendRequest(context.Context, transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	return nil, errors.New("not implemented")
}
func (t *closeTrackingTransport) SendNotification(context.Context, mcp.JSONRPCNotification) error {
	return nil
}
func (t *closeTrackingTransport) SetNotificationHandler(func(mcp.JSONRPCNotification)) {}
func (t *closeTrackingTransport) Close() error {
	t.closed.Store(true)
	t.closeCount.Add(1)
	return nil
}
func (t *closeTrackingTransport) GetSessionId() string { return "close-tracking" }

var _ transport.Interface = (*closeTrackingTransport)(nil)

func TestRemoveClient(t *testing.T) {
	// Removing a non-existent client should not panic
	empty := NewServerRegistry(nil)
	empty.RemoveClient("nonexistent")
	if empty.clients == nil {
		t.Error("Registry clients map should not be nil after RemoveClient")
	}

	const serverName = "mock-server"
	tr := &closeTrackingTransport{}
	mcpCli := mcpclient.NewClient(tr)
	wrapped := lazyclient.NewClientFromMCP(serverName, mcpCli)

	// Dummy config required so GetOrLoadServer can recreate after eviction
	cfg := &config.MCPClientConfigV2{
		TransportType: config.MCPClientTypeStdio,
		Command:       "true",
		Options:       &config.OptionsV2{},
	}
	registry := NewServerRegistry(map[string]*config.MCPClientConfigV2{
		serverName: cfg,
	})

	// Seed the registry with our close-tracking client
	registry.clients[serverName] = wrapped
	if _, ok := registry.clients[serverName]; !ok {
		t.Fatal("client should be present in registry before RemoveClient")
	}

	var createCount atomic.Int32
	var freshClient *lazyclient.Client
	registry.createClient = func(name string, conf *config.MCPClientConfigV2) (*lazyclient.Client, error) {
		createCount.Add(1)
		// Fresh transport that can answer Initialize (GetOrLoadServer always inits)
		initTr := newReconnectMockTransport(errors.New("broken pipe"), 0)
		initMcp := mcpclient.NewClient(initTr)
		freshClient = lazyclient.NewClientFromMCP(name, initMcp)
		return freshClient, nil
	}

	registry.RemoveClient(serverName)

	// (a) client removed from registry
	if _, ok := registry.clients[serverName]; ok {
		t.Error("client should be removed from registry after RemoveClient")
	}
	// (b) Close() called on the removed client
	if !tr.closed.Load() {
		t.Error("Close() should have been called on the removed client")
	}
	if got := tr.closeCount.Load(); got != 1 {
		t.Errorf("Close() call count = %d, want 1", got)
	}

	// (c) subsequent lookup creates a FRESH client
	ctx := context.Background()
	got, err := registry.GetOrLoadServer(ctx, serverName)
	if err != nil {
		t.Fatalf("GetOrLoadServer after RemoveClient: %v", err)
	}
	if createCount.Load() != 1 {
		t.Errorf("createClient call count = %d, want 1", createCount.Load())
	}
	if got == wrapped {
		t.Error("GetOrLoadServer should return a fresh client, not the removed instance")
	}
	if got != freshClient {
		t.Error("GetOrLoadServer should return the client created by the factory")
	}
	if registry.clients[serverName] != got {
		t.Error("fresh client should be stored in the registry")
	}
}
