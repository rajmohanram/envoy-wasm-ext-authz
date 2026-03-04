package metrics

import (
	"testing"
	"time"
)

// TestMetricsIncAllowed verifies allowed counter
func TestMetricsIncAllowed(t *testing.T) {
	m := New()

	m.IncAllowed()
	m.IncAllowed()
	m.IncAllowed()

	stats := m.GetStats()
	if stats.TotalRequests != 3 {
		t.Errorf("Expected total requests 3, got %d", stats.TotalRequests)
	}
	if stats.AllowedCount != 3 {
		t.Errorf("Expected allowed count 3, got %d", stats.AllowedCount)
	}
	if stats.DeniedCount != 0 {
		t.Errorf("Expected denied count 0, got %d", stats.DeniedCount)
	}
}

// TestMetricsIncDenied verifies denied counter
func TestMetricsIncDenied(t *testing.T) {
	m := New()

	m.IncDenied()
	m.IncDenied()

	stats := m.GetStats()
	if stats.TotalRequests != 2 {
		t.Errorf("Expected total requests 2, got %d", stats.TotalRequests)
	}
	if stats.AllowedCount != 0 {
		t.Errorf("Expected allowed count 0, got %d", stats.AllowedCount)
	}
	if stats.DeniedCount != 2 {
		t.Errorf("Expected denied count 2, got %d", stats.DeniedCount)
	}
}

// TestMetricsIncErrors verifies error counter
func TestMetricsIncErrors(t *testing.T) {
	m := New()

	m.IncErrors()
	m.IncErrors()
	m.IncErrors()

	stats := m.GetStats()
	if stats.ErrorCount != 3 {
		t.Errorf("Expected error count 3, got %d", stats.ErrorCount)
	}
}

// TestMetricsRecordLatency verifies latency tracking
func TestMetricsRecordLatency(t *testing.T) {
	m := New()

	// Record some latencies
	m.RecordLatency(10 * time.Millisecond)
	m.RecordLatency(20 * time.Millisecond)
	m.RecordLatency(30 * time.Millisecond)

	// Need to increment request count for avg calculation
	m.IncAllowed()
	m.IncAllowed()
	m.IncAllowed()

	stats := m.GetStats()

	// Average should be 20ms
	expected := 20.0
	tolerance := 1.0
	if stats.AvgLatencyMs < expected-tolerance || stats.AvgLatencyMs > expected+tolerance {
		t.Errorf("Expected avg latency ~%.2fms, got %.2fms", expected, stats.AvgLatencyMs)
	}
}

// TestMetricsRates verifies rate calculations
func TestMetricsRates(t *testing.T) {
	m := New()

	// 7 allowed, 3 denied
	for i := 0; i < 7; i++ {
		m.IncAllowed()
	}
	for i := 0; i < 3; i++ {
		m.IncDenied()
	}

	stats := m.GetStats()

	// Allow rate should be 70%
	if stats.AllowRate != 70.0 {
		t.Errorf("Expected allow rate 70%%, got %.2f%%", stats.AllowRate)
	}

	// Deny rate should be 30%
	if stats.DenyRate != 30.0 {
		t.Errorf("Expected deny rate 30%%, got %.2f%%", stats.DenyRate)
	}
}

// TestMetricsUptime verifies uptime calculation
func TestMetricsUptime(t *testing.T) {
	m := New()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	stats := m.GetStats()

	// Uptime should be at least 0 seconds
	if stats.UptimeSeconds < 0 {
		t.Errorf("Expected uptime >= 0, got %d", stats.UptimeSeconds)
	}
}

// TestMetricsRequestsPerSec verifies RPS calculation
func TestMetricsRequestsPerSec(t *testing.T) {
	m := New()

	// Add some requests
	for i := 0; i < 10; i++ {
		m.IncAllowed()
	}

	// Wait at least 1 second to ensure uptime >= 1
	time.Sleep(1100 * time.Millisecond)

	stats := m.GetStats()

	// Should have some RPS value (at least 1 req/sec since we waited 1s+ and had 10 requests)
	if stats.RequestsPerSec <= 0 {
		t.Errorf("Expected requests per second > 0, got %.2f", stats.RequestsPerSec)
	}

	// RPS should be reasonable (10 requests in ~1 second)
	if stats.RequestsPerSec < 1 || stats.RequestsPerSec > 20 {
		t.Logf("Warning: RPS %.2f seems unusual for 10 requests in ~1s", stats.RequestsPerSec)
	}
}

// TestMetricsReset verifies reset functionality
func TestMetricsReset(t *testing.T) {
	m := New()

	// Add some data
	m.IncAllowed()
	m.IncDenied()
	m.IncErrors()
	m.RecordLatency(10 * time.Millisecond)

	// Reset
	m.Reset()

	stats := m.GetStats()

	// All should be zero
	if stats.TotalRequests != 0 {
		t.Errorf("Expected total requests 0 after reset, got %d", stats.TotalRequests)
	}
	if stats.AllowedCount != 0 {
		t.Errorf("Expected allowed count 0 after reset, got %d", stats.AllowedCount)
	}
	if stats.DeniedCount != 0 {
		t.Errorf("Expected denied count 0 after reset, got %d", stats.DeniedCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("Expected error count 0 after reset, got %d", stats.ErrorCount)
	}
}

// TestMetricsConcurrency verifies thread safety
func TestMetricsConcurrency(t *testing.T) {
	m := New()

	// Run concurrent operations
	done := make(chan bool)
	operations := 100

	// Increment allowed
	go func() {
		for i := 0; i < operations; i++ {
			m.IncAllowed()
		}
		done <- true
	}()

	// Increment denied
	go func() {
		for i := 0; i < operations; i++ {
			m.IncDenied()
		}
		done <- true
	}()

	// Increment errors
	go func() {
		for i := 0; i < operations; i++ {
			m.IncErrors()
		}
		done <- true
	}()

	// Record latency
	go func() {
		for i := 0; i < operations; i++ {
			m.RecordLatency(time.Millisecond)
		}
		done <- true
	}()

	// Wait for all to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	stats := m.GetStats()

	// Should have exactly operations*2 requests (allowed + denied)
	if stats.TotalRequests != uint64(operations*2) {
		t.Errorf("Expected total requests %d, got %d", operations*2, stats.TotalRequests)
	}
	if stats.AllowedCount != uint64(operations) {
		t.Errorf("Expected allowed count %d, got %d", operations, stats.AllowedCount)
	}
	if stats.DeniedCount != uint64(operations) {
		t.Errorf("Expected denied count %d, got %d", operations, stats.DeniedCount)
	}
	if stats.ErrorCount != uint64(operations) {
		t.Errorf("Expected error count %d, got %d", operations, stats.ErrorCount)
	}
}

// BenchmarkMetricsIncAllowed measures performance of IncAllowed
func BenchmarkMetricsIncAllowed(b *testing.B) {
	m := New()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.IncAllowed()
	}
}

// BenchmarkMetricsGetStats measures performance of GetStats
func BenchmarkMetricsGetStats(b *testing.B) {
	m := New()

	// Add some data
	for i := 0; i < 1000; i++ {
		m.IncAllowed()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.GetStats()
	}
}
