package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCallMarshalJSON verifies JSON serialization outputs duration in milliseconds
func TestToolCallMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		call     ToolCall
		contains string
	}{
		{
			name:     "zero duration",
			call:     ToolCall{Duration: 0},
			contains: `"duration_ms":0`,
		},
		{
			name:     "1500ms duration",
			call:     ToolCall{Duration: 1500 * time.Millisecond},
			contains: `"duration_ms":1500`,
		},
		{
			name:     "with error field",
			call:     ToolCall{Duration: 500 * time.Millisecond, Error: "test error"},
			contains: `"duration_ms":500`,
		},
		{
			name:     "60 second duration",
			call:     ToolCall{Duration: 60 * time.Second},
			contains: `"duration_ms":60000`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.call)
			require.NoError(t, err)
			assert.Contains(t, string(data), tt.contains)
		})
	}
}

// TestToolCallUnmarshalJSON verifies JSON deserialization converts milliseconds back to Duration
// This test was added after PR review identified missing UnmarshalJSON as a bug
func TestToolCallUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected time.Duration
	}{
		{
			name:     "zero duration",
			json:     `{"ts":"2024-01-01T00:00:00Z","server":"test","tool_path":"test.tool","duration_ms":0,"success":true}`,
			expected: 0,
		},
		{
			name:     "1500ms",
			json:     `{"ts":"2024-01-01T00:00:00Z","server":"test","tool_path":"test.tool","duration_ms":1500,"success":true}`,
			expected: 1500 * time.Millisecond,
		},
		{
			name:     "60 seconds (60000ms)",
			json:     `{"ts":"2024-01-01T00:00:00Z","server":"test","tool_path":"test.tool","duration_ms":60000,"success":true}`,
			expected: 60 * time.Second,
		},
		{
			name:     "with error",
			json:     `{"ts":"2024-01-01T00:00:00Z","server":"test","tool_path":"test.tool","duration_ms":500,"success":false,"error":"timeout"}`,
			expected: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var call ToolCall
			err := json.Unmarshal([]byte(tt.json), &call)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, call.Duration, "Duration should be correctly parsed from milliseconds")
		})
	}
}

// TestSerializationRoundTrip verifies that marshal→unmarshal preserves all data
// Critical test: ensures persistence loading doesn't corrupt duration values
func TestSerializationRoundTrip(t *testing.T) {
	original := ToolCall{
		Timestamp: time.Now().Truncate(time.Second), // Truncate to avoid nanosecond precision issues
		Server:    "test-server",
		ToolPath:  "gmail.send_email",
		Duration:  1500 * time.Millisecond,
		Success:   true,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var restored ToolCall
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify all fields survive round-trip
	assert.Equal(t, original.Duration, restored.Duration, "Duration must survive round-trip")
	assert.Equal(t, original.Server, restored.Server)
	assert.Equal(t, original.ToolPath, restored.ToolPath)
	assert.Equal(t, original.Success, restored.Success)
}

// TestSerializationRoundTripWithError verifies error fields are preserved
func TestSerializationRoundTripWithError(t *testing.T) {
	original := ToolCall{
		Timestamp: time.Now().Truncate(time.Second),
		Server:    "gmail",
		ToolPath:  "gmail.send_email",
		Duration:  30 * time.Second,
		Success:   false,
		Error:     "context deadline exceeded",
		IsTimeout: true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored ToolCall
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.Error, restored.Error)
	assert.Equal(t, original.IsTimeout, restored.IsTimeout)
	assert.Equal(t, original.Duration, restored.Duration)
}

// TestPercentile verifies percentile calculation accuracy
func TestPercentile(t *testing.T) {
	tests := []struct {
		name      string
		durations []time.Duration
		p         float64
		expected  time.Duration
	}{
		{
			name:      "empty slice returns zero",
			durations: []time.Duration{},
			p:         0.50,
			expected:  0,
		},
		{
			name:      "single value returns that value",
			durations: []time.Duration{100 * time.Millisecond},
			p:         0.50,
			expected:  100 * time.Millisecond,
		},
		{
			name:      "p50 of sorted sequence",
			durations: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
			p:         0.50,
			expected:  30 * time.Millisecond,
		},
		{
			name:      "p0 returns minimum",
			durations: []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond},
			p:         0.0,
			expected:  100 * time.Millisecond,
		},
		{
			name:      "p100 returns maximum",
			durations: []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond},
			p:         1.0,
			expected:  300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := percentile(tt.durations, tt.p)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseDuration verifies duration string parsing including day notation
func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "standard hour",
			input:    "1h",
			expected: time.Hour,
			hasError: false,
		},
		{
			name:     "24 hours",
			input:    "24h",
			expected: 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "7 days",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "1 day",
			input:    "1d",
			expected: 24 * time.Hour,
			hasError: false,
		},
		{
			name:     "empty defaults to 1h",
			input:    "",
			expected: time.Hour,
			hasError: false,
		},
		{
			name:     "whitespace defaults to 1h",
			input:    "  ",
			expected: time.Hour,
			hasError: false,
		},
		{
			name:     "30 minutes",
			input:    "30m",
			expected: 30 * time.Minute,
			hasError: false,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			hasError: true,
		},
		{
			name:     "invalid day format",
			input:    "xd",
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestStoreRecordCall verifies basic call recording functionality
func TestStoreRecordCall(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	// Record successful call
	store.RecordCall("gmail", "send_email", 500*time.Millisecond, nil)

	// Record failed call
	store.RecordCall("gmail", "send_email", 600*time.Millisecond, errors.New("timeout"))

	calls := store.GetCallsInWindow("gmail", time.Hour)
	require.Len(t, calls, 2)

	assert.True(t, calls[0].Success)
	assert.Equal(t, 500*time.Millisecond, calls[0].Duration)

	assert.False(t, calls[1].Success)
	assert.Equal(t, "timeout", calls[1].Error)
}

// TestStoreRecordCallTimeoutDetection verifies timeout errors are flagged
func TestStoreRecordCallTimeoutDetection(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	// Record timeout error
	store.RecordCall("slow-server", "heavy_op", 30*time.Second, errors.New("context deadline exceeded"))

	calls := store.GetCallsInWindow("slow-server", time.Hour)
	require.Len(t, calls, 1)

	assert.False(t, calls[0].Success)
	assert.True(t, calls[0].IsTimeout, "Timeout errors should be flagged")
}

// TestStoreGetCallsInWindowFiltering verifies server filtering works
func TestStoreGetCallsInWindowFiltering(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	store.RecordCall("gmail", "send", 100*time.Millisecond, nil)
	store.RecordCall("asana", "create_task", 200*time.Millisecond, nil)
	store.RecordCall("gmail", "read", 150*time.Millisecond, nil)

	// Filter by gmail
	gmailCalls := store.GetCallsInWindow("gmail", time.Hour)
	assert.Len(t, gmailCalls, 2)

	// Filter by asana
	asanaCalls := store.GetCallsInWindow("asana", time.Hour)
	assert.Len(t, asanaCalls, 1)

	// Get all calls (empty filter)
	allCalls := store.GetCallsInWindow("", time.Hour)
	assert.Len(t, allCalls, 3)
}

// TestStoreServerStats verifies server load tracking
func TestStoreServerStats(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	// Record server load
	store.RecordServerLoad("gmail", 150) // 150ms cold start

	stats := store.GetServerStats("gmail")
	require.NotNil(t, stats)
	assert.Equal(t, "gmail", stats.Name)
	assert.Equal(t, int64(150), stats.ColdStartMs)
	assert.True(t, stats.IsConnected)

	// Non-existent server
	nilStats := store.GetServerStats("nonexistent")
	assert.Nil(t, nilStats)
}

// TestStoreServerDisconnect verifies disconnect tracking
func TestStoreServerDisconnect(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	store.RecordServerLoad("gmail", 100)
	assert.Contains(t, store.GetActiveServers(), "gmail")

	store.RecordServerDisconnect("gmail")

	stats := store.GetServerStats("gmail")
	require.NotNil(t, stats)
	assert.False(t, stats.IsConnected)
	assert.NotContains(t, store.GetActiveServers(), "gmail")
}

// TestStoreLifecycle verifies Start/Stop behavior
func TestStoreLifecycle(t *testing.T) {
	store := NewStore(100*time.Millisecond, "") // Short retention for testing

	// Record some calls
	store.RecordCall("server1", "tool1", 50*time.Millisecond, nil)
	store.RecordCall("server1", "tool2", 100*time.Millisecond, errors.New("failed"))

	calls := store.GetCallsInWindow("", time.Hour)
	assert.Len(t, calls, 2)

	// Stop should work without panic
	store.Stop()
}

// TestStoreConcurrency verifies thread safety under concurrent access
func TestStoreConcurrency(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.RecordCall("server", fmt.Sprintf("tool_%d", n), time.Duration(n)*time.Millisecond, nil)
		}(i)
	}

	// Concurrent reads while writes are happening
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetCallsInWindow("", time.Hour)
		}()
	}

	wg.Wait()

	// Verify all writes completed
	calls := store.GetCallsInWindow("", time.Hour)
	assert.Len(t, calls, numGoroutines)
}

// TestGetMetrics verifies aggregated metrics computation
func TestGetMetrics(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	// Add mixed results
	for i := 0; i < 8; i++ {
		store.RecordCall("server1", "tool", 100*time.Millisecond, nil)
	}
	store.RecordCall("server1", "tool", 100*time.Millisecond, errors.New("error1"))
	store.RecordCall("server1", "tool", 100*time.Millisecond, errors.New("error2"))

	store.RecordServerLoad("server1", 200)

	metrics := store.GetMetrics("", time.Hour)

	// Check summary
	summary, ok := metrics["summary"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 10, summary["total_calls"])
	assert.Equal(t, "80.0%", summary["success_rate"]) // 8/10 = 80%
	assert.Equal(t, 1, summary["active_servers"])

	// Check recent errors
	recentErrors, ok := metrics["recent_errors"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, recentErrors, 2) // 2 errors recorded
}

// TestFormatDuration verifies human-readable duration formatting
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{100 * time.Millisecond, "100ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
		{60 * time.Second, "60.0s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsTimeoutError verifies timeout error detection
func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context deadline exceeded", errors.New("context deadline exceeded"), true},
		{"timeout", errors.New("operation timeout"), true},
		{"timed out", errors.New("connection timed out"), true},
		{"regular error", errors.New("connection refused"), false},
		{"uppercase TIMEOUT", errors.New("TIMEOUT"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
