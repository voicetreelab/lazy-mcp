package hierarchy

import (
	"errors"
	"testing"
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
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "connection timed out",
			err:      errors.New("dial tcp: connection timed out"),
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
		// Should NOT detect as transport errors
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

func TestRemoveClient(t *testing.T) {
	// Create a registry with no servers (we just test the mechanics)
	registry := NewServerRegistry(nil)

	// Removing a non-existent client should not panic
	registry.RemoveClient("nonexistent")

	// The registry should still be functional
	if registry.clients == nil {
		t.Error("Registry clients map should not be nil after RemoveClient")
	}
}
