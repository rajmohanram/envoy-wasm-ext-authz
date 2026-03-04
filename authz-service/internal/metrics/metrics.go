// Package metrics provides thread-safe performance metrics tracking for the authorization service.
// It uses atomic operations for lock-free concurrent access in high-throughput scenarios.
//
// Tracked Metrics:
//   - Request counts (total, allowed, denied, errors)
//   - Average request latency
//   - Request rate (requests per second)
//   - Service uptime
//
// All operations are safe for concurrent use from multiple goroutines without external locking.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks service performance metrics using atomic operations for thread safety.
// All counters use atomic.Uint64 to avoid lock contention in high-concurrency scenarios.
type Metrics struct {
	totalRequests  atomic.Uint64 // Total number of authorization requests processed
	allowedCount   atomic.Uint64 // Number of requests allowed
	deniedCount    atomic.Uint64 // Number of requests denied
	errorCount     atomic.Uint64 // Number of errors encountered
	totalLatencyNs atomic.Uint64 // Sum of all request latencies in nanoseconds
	startTime      time.Time     // Service start time for uptime calculation
	mu             sync.RWMutex  // Protects startTime during Reset operations
}

// New creates and initializes a new Metrics instance.
// The metrics start time is set to the current time for uptime tracking.
func New() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// IncAllowed atomically increments both the total request counter and the allowed counter.
// Thread-safe and lock-free.
func (m *Metrics) IncAllowed() {
	m.totalRequests.Add(1)
	m.allowedCount.Add(1)
}

// IncDenied atomically increments both the total request counter and the denied counter.
// Thread-safe and lock-free.
func (m *Metrics) IncDenied() {
	m.totalRequests.Add(1)
	m.deniedCount.Add(1)
}

// IncErrors atomically increments the error counter.
// Thread-safe and lock-free.
func (m *Metrics) IncErrors() {
	m.errorCount.Add(1)
}

// RecordLatency atomically adds the request duration to the total latency accumulator.
// Thread-safe and lock-free.
func (m *Metrics) RecordLatency(duration time.Duration) {
	m.totalLatencyNs.Add(uint64(duration.Nanoseconds()))
}

// Stats represents a snapshot of service metrics at a point in time.
// All rate calculations are derived from the counter values and uptime.
type Stats struct {
	TotalRequests  uint64  `json:"total_requests"`   // Total requests processed
	AllowedCount   uint64  `json:"allowed_count"`    // Requests allowed
	DeniedCount    uint64  `json:"denied_count"`     // Requests denied
	ErrorCount     uint64  `json:"error_count"`      // Errors encountered
	AllowRate      float64 `json:"allow_rate"`       // Percentage of requests allowed (0-100)
	DenyRate       float64 `json:"deny_rate"`        // Percentage of requests denied (0-100)
	AvgLatencyMs   float64 `json:"avg_latency_ms"`   // Average request latency in milliseconds
	UptimeSeconds  int64   `json:"uptime_seconds"`   // Service uptime in seconds
	RequestsPerSec float64 `json:"requests_per_sec"` // Average requests per second
}

// GetStats returns a snapshot of current metrics with calculated rates.
// This method takes a read lock briefly to access the start time but reads
// all counters atomically without holding locks.
func (m *Metrics) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.totalRequests.Load()
	allowed := m.allowedCount.Load()
	denied := m.deniedCount.Load()
	errors := m.errorCount.Load()
	totalLatency := m.totalLatencyNs.Load()

	uptime := time.Since(m.startTime)
	uptimeSeconds := int64(uptime.Seconds())

	stats := &Stats{
		TotalRequests: total,
		AllowedCount:  allowed,
		DeniedCount:   denied,
		ErrorCount:    errors,
		UptimeSeconds: uptimeSeconds,
	}

	// Calculate rates
	if total > 0 {
		stats.AllowRate = float64(allowed) / float64(total) * 100
		stats.DenyRate = float64(denied) / float64(total) * 100
		stats.AvgLatencyMs = float64(totalLatency) / float64(total) / 1e6 // ns to ms
	}

	// Calculate requests per second
	if uptimeSeconds > 0 {
		stats.RequestsPerSec = float64(total) / float64(uptimeSeconds)
	}

	return stats
}

// Reset resets all metrics (useful for testing)
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests.Store(0)
	m.allowedCount.Store(0)
	m.deniedCount.Store(0)
	m.errorCount.Store(0)
	m.totalLatencyNs.Store(0)
	m.startTime = time.Now()
}
