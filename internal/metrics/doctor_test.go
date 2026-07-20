package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCheckServerConnectivity verifies server connectivity health check
func TestCheckServerConnectivity(t *testing.T) {
	tests := []struct {
		name            string
		loadServers     []string // Servers to load (will be marked connected)
		expectedServers []string // Servers the check expects to see
		expectedStatus  CheckStatus
	}{
		{
			name:            "all servers connected",
			loadServers:     []string{"gmail", "asana"},
			expectedServers: []string{"gmail", "asana"},
			expectedStatus:  CheckStatusPass,
		},
		{
			name:            "some servers disconnected",
			loadServers:     []string{"gmail"},
			expectedServers: []string{"gmail", "asana"},
			expectedStatus:  CheckStatusWarning,
		},
		{
			name:            "no servers connected",
			loadServers:     []string{},
			expectedServers: []string{"gmail", "asana"},
			expectedStatus:  CheckStatusWarning,
		},
		{
			name:            "no servers expected",
			loadServers:     []string{},
			expectedServers: []string{},
			expectedStatus:  CheckStatusPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(time.Hour, "")
			defer store.Stop()

			// Load servers
			for _, s := range tt.loadServers {
				store.RecordServerLoad(s, 100)
			}

			result := checkServerConnectivity(store, tt.expectedServers)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, "server_connectivity", result.Name)
		})
	}
}

// TestCheckLatency verifies latency health check thresholds
func TestCheckLatency(t *testing.T) {
	tests := []struct {
		name           string
		calls          []struct {
			server   string
			duration time.Duration
		}
		expectedStatus CheckStatus
	}{
		{
			name:           "no calls - pass",
			calls:          []struct{ server string; duration time.Duration }{},
			expectedStatus: CheckStatusPass,
		},
		{
			name: "all fast - pass",
			calls: []struct{ server string; duration time.Duration }{
				{"fast", 100 * time.Millisecond},
				{"fast", 200 * time.Millisecond},
			},
			expectedStatus: CheckStatusPass,
		},
		{
			name: "warning threshold exceeded",
			calls: []struct{ server string; duration time.Duration }{
				{"slow", 35 * time.Second}, // Above 30s warning threshold
			},
			expectedStatus: CheckStatusWarning,
		},
		{
			name: "critical threshold exceeded",
			calls: []struct{ server string; duration time.Duration }{
				{"critical", 50 * time.Second}, // Above 45s critical threshold
			},
			expectedStatus: CheckStatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(time.Hour, "")
			defer store.Stop()

			for _, call := range tt.calls {
				store.RecordCall(call.server, "tool", call.duration, nil)
			}

			result := checkLatency(store, nil)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, "latency", result.Name)
		})
	}
}

// TestCheckTimeouts verifies timeout detection health check
func TestCheckTimeouts(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*Store)
		expectedStatus CheckStatus
	}{
		{
			name:           "no calls - pass",
			setupFunc:      func(s *Store) {},
			expectedStatus: CheckStatusPass,
		},
		{
			name: "no timeouts - pass",
			setupFunc: func(s *Store) {
				s.RecordCall("server", "tool", 100*time.Millisecond, nil)
				s.RecordCall("server", "tool", 200*time.Millisecond, errors.New("regular error"))
			},
			expectedStatus: CheckStatusPass,
		},
		{
			name: "single timeout - warning",
			setupFunc: func(s *Store) {
				s.RecordCall("server", "tool", 100*time.Millisecond, nil)
				s.RecordCall("server", "tool", 30*time.Second, errors.New("context deadline exceeded"))
			},
			expectedStatus: CheckStatusWarning,
		},
		{
			name: "many timeouts - fail",
			setupFunc: func(s *Store) {
				for i := 0; i < 5; i++ {
					s.RecordCall("server", "tool", 30*time.Second, errors.New("timeout"))
				}
			},
			expectedStatus: CheckStatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(time.Hour, "")
			defer store.Stop()

			tt.setupFunc(store)

			result := checkTimeouts(store, nil)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, "timeouts", result.Name)
		})
	}
}

// TestCheckErrorRate verifies error rate health check thresholds
func TestCheckErrorRate(t *testing.T) {
	tests := []struct {
		name           string
		successCount   int
		errorCount     int
		expectedStatus CheckStatus
	}{
		{
			name:           "no calls - pass",
			successCount:   0,
			errorCount:     0,
			expectedStatus: CheckStatusPass,
		},
		{
			name:           "100% success - pass",
			successCount:   100,
			errorCount:     0,
			expectedStatus: CheckStatusPass,
		},
		{
			name:           "4% error rate - pass",
			successCount:   96,
			errorCount:     4,
			expectedStatus: CheckStatusPass,
		},
		{
			name:           "6% error rate - warning",
			successCount:   94,
			errorCount:     6,
			expectedStatus: CheckStatusWarning,
		},
		{
			name:           "11% error rate - fail",
			successCount:   89,
			errorCount:     11,
			expectedStatus: CheckStatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(time.Hour, "")
			defer store.Stop()

			for i := 0; i < tt.successCount; i++ {
				store.RecordCall("server", "tool", 100*time.Millisecond, nil)
			}
			for i := 0; i < tt.errorCount; i++ {
				store.RecordCall("server", "tool", 100*time.Millisecond, errors.New("error"))
			}

			result := checkErrorRate(store, nil)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, "error_rate", result.Name)
		})
	}
}

// TestCheckMemory verifies memory usage reporting
func TestCheckMemory(t *testing.T) {
	store := NewStore(time.Hour, "")
	defer store.Stop()

	// Add some calls
	for i := 0; i < 100; i++ {
		store.RecordCall("server", "tool", 100*time.Millisecond, nil)
	}

	result := checkMemory(store, nil)

	// Memory check should always pass (informational only)
	assert.Equal(t, CheckStatusPass, result.Status)
	assert.Equal(t, "memory", result.Name)
	assert.Contains(t, result.Message, "100 records")
}

// TestRunDoctor verifies full doctor diagnostics
func TestRunDoctor(t *testing.T) {
	t.Run("healthy state", func(t *testing.T) {
		store := NewStore(time.Hour, "")
		defer store.Stop()

		store.RecordServerLoad("server1", 100)
		store.RecordCall("server1", "tool", 100*time.Millisecond, nil)

		result := RunDoctor(store, []string{"server1"})

		assert.Equal(t, CheckStatusPass, result.Status)
		assert.Len(t, result.Checks, 5, "Should run all 5 checks")
	})

	t.Run("unhealthy state aggregates worst status", func(t *testing.T) {
		store := NewStore(time.Hour, "")
		defer store.Stop()

		// Add critical latency
		store.RecordCall("slow", "tool", 50*time.Second, nil)

		result := RunDoctor(store, []string{"server1"})

		assert.Equal(t, CheckStatusFail, result.Status, "Overall status should be worst individual status")
	})

	t.Run("suggestions are collected", func(t *testing.T) {
		store := NewStore(time.Hour, "")
		defer store.Stop()

		// Create conditions that generate suggestions
		store.RecordCall("server", "tool", 30*time.Second, errors.New("context deadline exceeded"))

		result := RunDoctor(store, []string{"missing-server"})

		assert.NotEmpty(t, result.Suggestions, "Should have suggestions for issues found")
	})
}

// TestCheckStatusPriority verifies status aggregation priority
func TestCheckStatusPriority(t *testing.T) {
	// Verify fail > warning > pass priority
	tests := []struct {
		name           string
		checkStatuses  []CheckStatus
		expectedOverall CheckStatus
	}{
		{
			name:           "all pass",
			checkStatuses:  []CheckStatus{CheckStatusPass, CheckStatusPass, CheckStatusPass},
			expectedOverall: CheckStatusPass,
		},
		{
			name:           "one warning among passes",
			checkStatuses:  []CheckStatus{CheckStatusPass, CheckStatusWarning, CheckStatusPass},
			expectedOverall: CheckStatusWarning,
		},
		{
			name:           "one fail overrides warnings",
			checkStatuses:  []CheckStatus{CheckStatusWarning, CheckStatusFail, CheckStatusWarning},
			expectedOverall: CheckStatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the status aggregation logic from RunDoctor
			overall := CheckStatusPass
			for _, status := range tt.checkStatuses {
				if status == CheckStatusFail && overall != CheckStatusFail {
					overall = CheckStatusFail
				} else if status == CheckStatusWarning && overall == CheckStatusPass {
					overall = CheckStatusWarning
				}
			}
			assert.Equal(t, tt.expectedOverall, overall)
		})
	}
}

// TestUniqueHelper verifies the unique helper function
func TestUniqueHelper(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unique(tt.input)
			assert.Len(t, result, len(tt.expected))
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

// TestJoinStringsHelper verifies the joinStrings helper function
func TestJoinStringsHelper(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		sep      string
		expected string
	}{
		{
			name:     "empty slice",
			items:    []string{},
			sep:      "; ",
			expected: "",
		},
		{
			name:     "single item",
			items:    []string{"one"},
			sep:      "; ",
			expected: "one",
		},
		{
			name:     "multiple items",
			items:    []string{"one", "two", "three"},
			sep:      "; ",
			expected: "one; two; three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinStrings(tt.items, tt.sep)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestThresholdConstants verifies threshold values are reasonable
func TestThresholdConstants(t *testing.T) {
	// Warning should be less than critical
	assert.Less(t, LatencyWarningThreshold, LatencyCriticalThreshold)
	assert.Less(t, ErrorRateWarningThreshold, ErrorRateCriticalThreshold)

	// Thresholds should be positive
	assert.Greater(t, LatencyWarningThreshold, time.Duration(0))
	assert.Greater(t, LatencyCriticalThreshold, time.Duration(0))
	assert.Greater(t, ErrorRateWarningThreshold, float64(0))
	assert.Greater(t, ErrorRateCriticalThreshold, float64(0))
	assert.Greater(t, TimeoutCountThreshold, 0)
}
