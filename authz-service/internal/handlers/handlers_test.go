package handlers

import (
	"encoding/json"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/metrics"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthzHandlerFailOpen verifies fail-open mode
func TestAuthzHandlerFailOpen(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false) // fail_open=true, no WAF

	req := httptest.NewRequest("POST", "/v1/test", strings.NewReader(`{"test":"data"}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var resp types.AuthzResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be allowed
	if !resp.Allowed {
		t.Error("Expected request to be allowed in fail-open mode")
	}
}

// TestAuthzHandlerFailClosed verifies fail-closed mode
func TestAuthzHandlerFailClosed(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, false, nil, false) // fail_open=false, no WAF

	req := httptest.NewRequest("POST", "/v1/test", strings.NewReader(`{"test":"data"}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200 (response format, not denial)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var resp types.AuthzResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be denied
	if resp.Allowed {
		t.Error("Expected request to be denied in fail-closed mode")
	}
}

// TestAuthzHandlerMaxBodySize verifies that body size limits don't affect streaming
// NOTE: Body is now streamed to WAF without reading in authz-service,
// so body size validation happens downstream (in WAF or backend)
func TestAuthzHandlerMaxBodySize(t *testing.T) {
	log := logger.New("debug")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false) // fail_open=true

	// Create large body
	largeBody := strings.Repeat("x", 200)
	req := httptest.NewRequest("POST", "/v1/test", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should succeed in fail-open mode (body is streamed, not validated here)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (streaming mode), got %d", w.Code)
	}

	// Parse response
	var resp types.AuthzResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be allowed (fail-open mode)
	if !resp.Allowed {
		t.Error("Expected request to be allowed in fail-open mode")
	}
}

// TestAuthzHandlerMetrics verifies metrics tracking
func TestAuthzHandlerMetrics(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false)

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/v1/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	stats := m.GetStats()

	// Should have 5 allowed requests
	if stats.AllowedCount != 5 {
		t.Errorf("Expected 5 allowed requests, got %d", stats.AllowedCount)
	}
}

// TestHealthHandler verifies health check
func TestHealthHandler(t *testing.T) {
	log := logger.New("error")
	handler := NewHealthHandler(log)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have status field
	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", resp["status"])
	}
}

// TestMetricsHandler verifies metrics endpoint
func TestMetricsHandler(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()

	// Add some data
	m.IncAllowed()
	m.IncDenied()

	handler := NewMetricsHandler(m, log)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var stats metrics.Stats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have metrics
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", stats.TotalRequests)
	}
}

// TestAuthzHandlerDifferentMethods verifies multiple HTTP methods
func TestAuthzHandlerDifferentMethods(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Method %s: Expected status 200, got %d", method, w.Code)
			}
		})
	}
}

// TestAuthzHandlerEmptyBody verifies empty body handling
func TestAuthzHandlerEmptyBody(t *testing.T) {
	log := logger.New("debug")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false)

	req := httptest.NewRequest("POST", "/v1/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should succeed with empty body
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// BenchmarkAuthzHandler measures handler performance
func BenchmarkAuthzHandler(b *testing.B) {
	log := logger.New("error") // Minimal logging
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false)

	req := httptest.NewRequest("POST", "/v1/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// BenchmarkAuthzHandlerWithBody measures handler with body
func BenchmarkAuthzHandlerWithBody(b *testing.B) {
	log := logger.New("error")
	m := metrics.New()
	handler := NewAuthzHandler(log, m, true, nil, false)

	body := `{"user":"test","action":"read","resource":"document"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/test", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
