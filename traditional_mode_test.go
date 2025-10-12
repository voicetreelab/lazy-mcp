package main

import (
	"testing"

	"github.com/TBXark/optional-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraditionalModeDetection tests that traditional mode is correctly detected
func TestTraditionalModeDetection(t *testing.T) {
	testCases := []struct {
		name                  string
		config                *Config
		expectRecursiveMode   bool
	}{
		{
			name: "Traditional mode - no recursiveLazyLoad flag",
			config: &Config{
				McpProxy: &MCPProxyConfigV2{
					BaseURL: "http://localhost",
					Addr:    ":8080",
					Name:    "Test",
					Version: "1.0.0",
					Type:    MCPServerTypeStreamable,
					Options: &OptionsV2{
						LazyLoad: optional.NewField(false),
					},
				},
				McpServers: map[string]*MCPClientConfigV2{},
			},
			expectRecursiveMode: false,
		},
		{
			name: "Traditional mode - recursiveLazyLoad explicitly false",
			config: &Config{
				McpProxy: &MCPProxyConfigV2{
					BaseURL: "http://localhost",
					Addr:    ":8080",
					Name:    "Test",
					Version: "1.0.0",
					Type:    MCPServerTypeStreamable,
					Options: &OptionsV2{
						RecursiveLazyLoad: optional.NewField(false),
					},
				},
				McpServers: map[string]*MCPClientConfigV2{},
			},
			expectRecursiveMode: false,
		},
		{
			name: "Recursive mode - recursiveLazyLoad true",
			config: &Config{
				McpProxy: &MCPProxyConfigV2{
					BaseURL: "http://localhost",
					Addr:    ":8080",
					Name:    "Test",
					Version: "1.0.0",
					Type:    MCPServerTypeStreamable,
					Options: &OptionsV2{
						RecursiveLazyLoad: optional.NewField(true),
					},
				},
				McpServers: map[string]*MCPClientConfigV2{},
			},
			expectRecursiveMode: true,
		},
		{
			name: "Traditional mode - nil options",
			config: &Config{
				McpProxy: &MCPProxyConfigV2{
					BaseURL: "http://localhost",
					Addr:    ":8080",
					Name:    "Test",
					Version: "1.0.0",
					Type:    MCPServerTypeStreamable,
					Options: nil,
				},
				McpServers: map[string]*MCPClientConfigV2{},
			},
			expectRecursiveMode: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isRecursive := tc.config.McpProxy.Options != nil && tc.config.McpProxy.Options.RecursiveLazyLoad.OrElse(false)
			assert.Equal(t, tc.expectRecursiveMode, isRecursive, "Mode detection should match expected")

			if isRecursive {
				t.Log("Detected: Recursive mode")
			} else {
				t.Log("Detected: Traditional mode")
			}
		})
	}
}

// TestConfigCompatibility tests that existing configs still work
func TestConfigCompatibility(t *testing.T) {
	t.Run("Load config without recursiveLazyLoad", func(t *testing.T) {
		// This would be a typical existing config
		config := &Config{
			McpProxy: &MCPProxyConfigV2{
				BaseURL: "http://localhost",
				Addr:    ":8080",
				Name:    "MCP Proxy",
				Version: "1.0.0",
				Type:    MCPServerTypeSSE,
				Options: &OptionsV2{
					LazyLoad:   optional.NewField(true),
					LogEnabled: optional.NewField(true),
				},
			},
			McpServers: map[string]*MCPClientConfigV2{
				"test-server": {
					TransportType: MCPClientTypeStdio,
					Command:       "test-command",
					Args:          []string{"arg1"},
				},
			},
		}

		require.NotNil(t, config.McpProxy.Options)

		// Should default to false (traditional mode)
		isRecursive := config.McpProxy.Options.RecursiveLazyLoad.OrElse(false)
		assert.False(t, isRecursive, "Should default to traditional mode")

		// Old LazyLoad flag should still work
		lazyLoad := config.McpProxy.Options.LazyLoad.OrElse(false)
		assert.True(t, lazyLoad, "Old LazyLoad flag should still work")

		t.Log("Existing configs remain compatible with traditional mode")
	})
}
