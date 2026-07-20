package metrics

import (
	"fmt"
	"time"
)

// CheckStatus represents the result status of a health check
type CheckStatus string

const (
	CheckStatusPass    CheckStatus = "pass"
	CheckStatusWarning CheckStatus = "warning"
	CheckStatusFail    CheckStatus = "fail"
)

// Check represents a single health check result
type Check struct {
	Name       string                 `json:"name"`
	Status     CheckStatus            `json:"status"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Suggestion string                 `json:"suggestion,omitempty"`
}

// DoctorResult is the response from proxy_doctor
type DoctorResult struct {
	Status      CheckStatus `json:"status"`
	Checks      []Check     `json:"checks"`
	Suggestions []string    `json:"suggestions"`
}

// Thresholds for health checks
const (
	LatencyWarningThreshold  = 30 * time.Second
	LatencyCriticalThreshold = 45 * time.Second
	ErrorRateWarningThreshold  = 5.0  // percent
	ErrorRateCriticalThreshold = 10.0 // percent
	TimeoutCountThreshold    = 2
)

// RunDoctor runs all health checks and returns diagnostics
func RunDoctor(store *Store, expectedServers []string) *DoctorResult {
	result := &DoctorResult{
		Status:      CheckStatusPass,
		Checks:      make([]Check, 0),
		Suggestions: make([]string, 0),
	}

	// Run all checks
	checks := []func(*Store, []string) Check{
		checkServerConnectivity,
		checkLatency,
		checkTimeouts,
		checkErrorRate,
		checkMemory,
	}

	for _, check := range checks {
		c := check(store, expectedServers)
		result.Checks = append(result.Checks, c)

		// Update overall status (fail > warning > pass)
		if c.Status == CheckStatusFail && result.Status != CheckStatusFail {
			result.Status = CheckStatusFail
		} else if c.Status == CheckStatusWarning && result.Status == CheckStatusPass {
			result.Status = CheckStatusWarning
		}

		// Collect suggestions
		if c.Suggestion != "" {
			result.Suggestions = append(result.Suggestions, c.Suggestion)
		}
	}

	return result
}

// checkServerConnectivity verifies all expected servers are connected
func checkServerConnectivity(store *Store, expectedServers []string) Check {
	activeServers := store.GetActiveServers()
	activeMap := make(map[string]bool)
	for _, s := range activeServers {
		activeMap[s] = true
	}

	var disconnected []string
	for _, expected := range expectedServers {
		if !activeMap[expected] {
			disconnected = append(disconnected, expected)
		}
	}

	if len(disconnected) > 0 {
		return Check{
			Name:    "server_connectivity",
			Status:  CheckStatusWarning,
			Message: fmt.Sprintf("%d of %d servers not yet loaded: %v", len(disconnected), len(expectedServers), disconnected),
			Details: map[string]interface{}{
				"active":       activeServers,
				"disconnected": disconnected,
			},
			Suggestion: "Disconnected servers will load on first tool call (lazy loading is enabled)",
		}
	}

	return Check{
		Name:    "server_connectivity",
		Status:  CheckStatusPass,
		Message: fmt.Sprintf("All %d servers responding", len(activeServers)),
	}
}

// checkLatency checks for high latency servers
func checkLatency(store *Store, _ []string) Check {
	calls := store.GetCallsInWindow("", time.Hour)
	if len(calls) == 0 {
		return Check{
			Name:    "latency",
			Status:  CheckStatusPass,
			Message: "No calls in the last hour to measure",
		}
	}

	// Group by server and find p99
	serverCalls := make(map[string][]time.Duration)
	for _, call := range calls {
		serverCalls[call.Server] = append(serverCalls[call.Server], call.Duration)
	}

	var warnings []string
	var details []map[string]interface{}

	for server, durations := range serverCalls {
		p99 := percentile(durations, 0.99)
		if p99 >= LatencyCriticalThreshold {
			warnings = append(warnings, fmt.Sprintf("%s p99=%.1fs (critical)", server, p99.Seconds()))
			details = append(details, map[string]interface{}{
				"server":    server,
				"p99":       fmt.Sprintf("%.1fs", p99.Seconds()),
				"threshold": fmt.Sprintf("%.0fs", LatencyCriticalThreshold.Seconds()), // Show critical threshold
			})
		} else if p99 >= LatencyWarningThreshold {
			warnings = append(warnings, fmt.Sprintf("%s p99=%.1fs", server, p99.Seconds()))
			details = append(details, map[string]interface{}{
				"server":    server,
				"p99":       fmt.Sprintf("%.1fs", p99.Seconds()),
				"threshold": fmt.Sprintf("%.0fs", LatencyWarningThreshold.Seconds()),
			})
		}
	}

	if len(warnings) > 0 {
		status := CheckStatusWarning
		for _, d := range details {
			p99Str := d["p99"].(string)
			// Check if any are critical
			var p99Val float64
			fmt.Sscanf(p99Str, "%fs", &p99Val)
			if p99Val >= LatencyCriticalThreshold.Seconds() {
				status = CheckStatusFail
				break
			}
		}

		return Check{
			Name:    "latency",
			Status:  status,
			Message: fmt.Sprintf("High latency detected: %v", warnings),
			Details: map[string]interface{}{
				"servers": details,
			},
			Suggestion: "Consider using smaller query parameters or increasing timeout for affected servers",
		}
	}

	return Check{
		Name:    "latency",
		Status:  CheckStatusPass,
		Message: "All servers within latency thresholds",
	}
}

// checkTimeouts checks for recent timeout events
func checkTimeouts(store *Store, _ []string) Check {
	calls := store.GetCallsInWindow("", time.Hour)

	// Count timeouts by server
	timeoutsByServer := make(map[string]int)
	toolsByServer := make(map[string][]string)
	for _, call := range calls {
		if call.IsTimeout {
			timeoutsByServer[call.Server]++
			toolsByServer[call.Server] = append(toolsByServer[call.Server], call.ToolPath)
		}
	}

	if len(timeoutsByServer) == 0 {
		return Check{
			Name:    "timeouts",
			Status:  CheckStatusPass,
			Message: "No timeouts in the last hour",
		}
	}

	var messages []string
	var suggestions []string
	totalTimeouts := 0

	for server, count := range timeoutsByServer {
		totalTimeouts += count
		messages = append(messages, fmt.Sprintf("%s: %d timeout(s)", server, count))

		// Generate specific suggestions based on server
		if len(toolsByServer[server]) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("%s: Consider using smaller parameters for %v", server, unique(toolsByServer[server])))
		}
	}

	status := CheckStatusWarning
	if totalTimeouts >= TimeoutCountThreshold*2 {
		status = CheckStatusFail
	}

	return Check{
		Name:    "timeouts",
		Status:  status,
		Message: fmt.Sprintf("%d timeout(s) in last hour: %v", totalTimeouts, messages),
		Details: map[string]interface{}{
			"by_server":      timeoutsByServer,
			"affected_tools": toolsByServer,
		},
		Suggestion: joinStrings(suggestions, "; "),
	}
}

// checkErrorRate checks the overall error rate
func checkErrorRate(store *Store, _ []string) Check {
	calls := store.GetCallsInWindow("", time.Hour)
	if len(calls) == 0 {
		return Check{
			Name:    "error_rate",
			Status:  CheckStatusPass,
			Message: "No calls in the last hour to measure",
		}
	}

	errorCount := 0
	for _, call := range calls {
		if !call.Success {
			errorCount++
		}
	}

	errorRate := float64(errorCount) / float64(len(calls)) * 100

	if errorRate >= ErrorRateCriticalThreshold {
		return Check{
			Name:    "error_rate",
			Status:  CheckStatusFail,
			Message: fmt.Sprintf("Error rate %.1f%% exceeds critical threshold (%.0f%%)", errorRate, ErrorRateCriticalThreshold),
			Details: map[string]interface{}{
				"error_count": errorCount,
				"total_calls": len(calls),
				"error_rate":  fmt.Sprintf("%.1f%%", errorRate),
			},
			Suggestion: "Check recent_errors in get_proxy_metrics for details on failing tools",
		}
	}

	if errorRate >= ErrorRateWarningThreshold {
		return Check{
			Name:    "error_rate",
			Status:  CheckStatusWarning,
			Message: fmt.Sprintf("Error rate %.1f%% exceeds warning threshold (%.0f%%)", errorRate, ErrorRateWarningThreshold),
			Details: map[string]interface{}{
				"error_count": errorCount,
				"total_calls": len(calls),
				"error_rate":  fmt.Sprintf("%.1f%%", errorRate),
			},
		}
	}

	return Check{
		Name:    "error_rate",
		Status:  CheckStatusPass,
		Message: fmt.Sprintf("Error rate %.1f%% (threshold: %.0f%%)", errorRate, ErrorRateWarningThreshold),
	}
}

// checkMemory reports metrics store memory usage (placeholder - always passes)
func checkMemory(store *Store, _ []string) Check {
	calls := store.GetCallsInWindow("", 24*time.Hour)
	// Rough estimate: ~200 bytes per call
	estimatedMB := float64(len(calls)*200) / 1024 / 1024

	return Check{
		Name:    "memory",
		Status:  CheckStatusPass,
		Message: fmt.Sprintf("Metrics store using ~%.1fMB (%d records)", estimatedMB, len(calls)),
	}
}

// Helper functions

func unique(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += sep + items[i]
	}
	return result
}
