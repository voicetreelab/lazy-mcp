// Package metrics provides observability for the MCP proxy.
// It tracks tool call latencies, success/failure rates, and server health.
// Metrics are stored in-memory with optional JSON persistence.
package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ToolCall represents a single tool invocation with timing and status
type ToolCall struct {
	Timestamp time.Time     `json:"ts"`
	Server    string        `json:"server"`
	ToolPath  string        `json:"tool_path"`
	Duration  time.Duration `json:"duration_ms"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	IsTimeout bool          `json:"is_timeout,omitempty"`
}

// MarshalJSON custom marshaler to output duration in milliseconds
func (tc ToolCall) MarshalJSON() ([]byte, error) {
	type Alias ToolCall
	return json.Marshal(&struct {
		Alias
		Duration int64 `json:"duration_ms"`
	}{
		Alias:    Alias(tc),
		Duration: tc.Duration.Milliseconds(),
	})
}

// ServerStats tracks server-level statistics
type ServerStats struct {
	Name        string    `json:"name"`
	LoadedAt    time.Time `json:"loaded_at"`
	ColdStartMs int64     `json:"cold_start_ms"`
	IsConnected bool      `json:"is_connected"`
}

// Store is the in-memory metrics store with optional JSON persistence
type Store struct {
	mu          sync.RWMutex
	calls       []ToolCall
	servers     map[string]*ServerStats
	retention   time.Duration
	persistPath string
	stopCh      chan struct{}
}

// NewStore creates a new metrics store
func NewStore(retention time.Duration, persistPath string) *Store {
	s := &Store{
		calls:       make([]ToolCall, 0),
		servers:     make(map[string]*ServerStats),
		retention:   retention,
		persistPath: persistPath,
		stopCh:      make(chan struct{}),
	}
	if persistPath != "" {
		s.loadFromFile()
	}
	go s.pruneLoop()
	return s
}

// RecordCall records a tool call with timing and result
func (s *Store) RecordCall(server, toolPath string, duration time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	call := ToolCall{
		Timestamp: time.Now(),
		Server:    server,
		ToolPath:  toolPath,
		Duration:  duration,
		Success:   err == nil,
	}
	if err != nil {
		call.Error = err.Error()
		call.IsTimeout = isTimeoutError(err)
	}
	s.calls = append(s.calls, call)
}

// RecordServerLoad records a server cold-start event
func (s *Store) RecordServerLoad(name string, coldStartMs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers[name] = &ServerStats{
		Name:        name,
		LoadedAt:    time.Now(),
		ColdStartMs: coldStartMs,
		IsConnected: true,
	}
}

// RecordServerDisconnect records a server disconnection
func (s *Store) RecordServerDisconnect(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stats, exists := s.servers[name]; exists {
		stats.IsConnected = false
	}
}

// GetMetrics returns metrics for the specified time window and optional server filter
func (s *Store) GetMetrics(serverFilter string, since time.Duration) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-since)
	result := make(map[string]interface{})

	// Filter calls by time window
	var filteredCalls []ToolCall
	for _, call := range s.calls {
		if call.Timestamp.After(cutoff) {
			if serverFilter == "" || call.Server == serverFilter {
				filteredCalls = append(filteredCalls, call)
			}
		}
	}

	// Compute summary
	totalCalls := len(filteredCalls)
	successCount := 0
	timeoutCount := 0
	for _, call := range filteredCalls {
		if call.Success {
			successCount++
		}
		if call.IsTimeout {
			timeoutCount++
		}
	}

	successRate := "0%"
	if totalCalls > 0 {
		rate := float64(successCount) / float64(totalCalls) * 100
		successRate = fmt.Sprintf("%.1f%%", rate)
	}

	// Count active servers
	activeServers := 0
	for _, stats := range s.servers {
		if stats.IsConnected {
			activeServers++
		}
	}

	result["time_window"] = formatDuration(since)
	result["summary"] = map[string]interface{}{
		"total_calls":    totalCalls,
		"success_rate":   successRate,
		"timeout_count":  timeoutCount,
		"active_servers": activeServers,
	}

	// Group by server
	byServer := make(map[string]map[string]interface{})
	serverCalls := make(map[string][]ToolCall)
	for _, call := range filteredCalls {
		serverCalls[call.Server] = append(serverCalls[call.Server], call)
	}

	for server, calls := range serverCalls {
		serverMetrics := computeServerMetrics(calls)
		byServer[server] = serverMetrics
	}
	result["by_server"] = byServer

	// Recent errors (last 10)
	var recentErrors []map[string]interface{}
	for i := len(filteredCalls) - 1; i >= 0 && len(recentErrors) < 10; i-- {
		call := filteredCalls[i]
		if !call.Success {
			recentErrors = append(recentErrors, map[string]interface{}{
				"time":   call.Timestamp.Format(time.RFC3339),
				"server": call.Server,
				"tool":   call.ToolPath,
				"error":  call.Error,
			})
		}
	}
	result["recent_errors"] = recentErrors

	return result
}

// GetActiveServers returns list of currently connected servers
func (s *Store) GetActiveServers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active []string
	for name, stats := range s.servers {
		if stats.IsConnected {
			active = append(active, name)
		}
	}
	return active
}

// GetServerStats returns stats for a specific server
func (s *Store) GetServerStats(name string) *ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if stats, exists := s.servers[name]; exists {
		statsCopy := *stats
		return &statsCopy
	}
	return nil
}

// GetCallsInWindow returns all calls in the time window for a server
func (s *Store) GetCallsInWindow(server string, since time.Duration) []ToolCall {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-since)
	var result []ToolCall
	for _, call := range s.calls {
		if call.Timestamp.After(cutoff) && (server == "" || call.Server == server) {
			result = append(result, call)
		}
	}
	return result
}

// Stop stops the background prune goroutine
func (s *Store) Stop() {
	close(s.stopCh)
}

// pruneLoop periodically removes old entries
func (s *Store) pruneLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.prune()
			if s.persistPath != "" {
				s.saveToFile()
			}
		case <-s.stopCh:
			return
		}
	}
}

// prune removes entries older than retention period
func (s *Store) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.retention)
	var kept []ToolCall
	for _, call := range s.calls {
		if call.Timestamp.After(cutoff) {
			kept = append(kept, call)
		}
	}
	s.calls = kept
}

// loadFromFile loads metrics from the persistence file
func (s *Store) loadFromFile() {
	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		return // File doesn't exist or can't be read
	}

	var saved struct {
		Calls   []ToolCall              `json:"calls"`
		Servers map[string]*ServerStats `json:"servers"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	s.calls = saved.Calls
	s.servers = saved.Servers

	// Mark all servers as disconnected on load (they need to reconnect)
	for _, stats := range s.servers {
		stats.IsConnected = false
	}
}

// saveToFile persists metrics to the file
func (s *Store) saveToFile() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	saved := struct {
		Calls   []ToolCall              `json:"calls"`
		Servers map[string]*ServerStats `json:"servers"`
	}{
		Calls:   s.calls,
		Servers: s.servers,
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(s.persistPath, data, 0644)
}

// Helper functions

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "timed out")
}

func computeServerMetrics(calls []ToolCall) map[string]interface{} {
	successCount := 0
	timeoutCount := 0
	var durations []time.Duration
	var lastError string
	slowestTool := ""
	var slowestDuration time.Duration

	for _, call := range calls {
		if call.Success {
			successCount++
		}
		if call.IsTimeout {
			timeoutCount++
		}
		if !call.Success && call.Error != "" {
			lastError = call.Error
		}
		durations = append(durations, call.Duration)
		if call.Duration > slowestDuration {
			slowestDuration = call.Duration
			slowestTool = call.ToolPath
		}
	}

	result := map[string]interface{}{
		"calls":    len(calls),
		"success":  successCount,
		"timeouts": timeoutCount,
	}

	if len(durations) > 0 {
		result["latency"] = map[string]string{
			"p50": formatDuration(percentile(durations, 0.50)),
			"p95": formatDuration(percentile(durations, 0.95)),
			"p99": formatDuration(percentile(durations, 0.99)),
		}
	}

	if slowestTool != "" {
		result["slowest_tool"] = slowestTool
	}
	if lastError != "" {
		result["last_error"] = lastError
	}

	return result
}

func percentile(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// Singleton for global metrics store
var (
	globalStore *Store
	globalOnce  sync.Once
)

// InitGlobalStore initializes the global metrics store
func InitGlobalStore(retention time.Duration, persistPath string) *Store {
	globalOnce.Do(func() {
		globalStore = NewStore(retention, persistPath)
	})
	return globalStore
}

// GetGlobalStore returns the global metrics store (or nil if not initialized)
func GetGlobalStore() *Store {
	return globalStore
}

// ParseDuration parses a duration string like "1h", "24h", "7d"
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Hour, nil // default
	}

	// Handle days specially
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
