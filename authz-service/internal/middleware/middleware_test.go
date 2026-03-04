package middleware

import (
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/metrics"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPanicRecovery verifies panic recovery middleware
func TestPanicRecovery(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with panic recovery
	handler := PanicRecovery(log, m)(panicHandler)

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	// Should return 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Should increment error count
	stats := m.GetStats()
	if stats.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", stats.ErrorCount)
	}
}

// TestPanicRecoveryNoPanic verifies normal operation
func TestPanicRecoveryNoPanic(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()

	// Normal handler
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("success")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Wrap with panic recovery
	handler := PanicRecovery(log, m)(normalHandler)

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should not increment error count
	stats := m.GetStats()
	if stats.ErrorCount != 0 {
		t.Errorf("Expected error count 0, got %d", stats.ErrorCount)
	}

	// Should have correct body
	if w.Body.String() != "success" {
		t.Errorf("Expected body 'success', got '%s'", w.Body.String())
	}
}

// TestPanicRecoveryMultiplePanics verifies multiple panics are handled
func TestPanicRecoveryMultiplePanics(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic " + r.URL.Path)
	})

	// Wrap with panic recovery
	handler := PanicRecovery(log, m)(panicHandler)

	// Make multiple requests
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Request %d: Expected status 500, got %d", i, w.Code)
		}
	}

	// Should have 5 errors
	stats := m.GetStats()
	if stats.ErrorCount != 5 {
		t.Errorf("Expected error count 5, got %d", stats.ErrorCount)
	}
}

// TestPanicRecoveryDifferentPanicTypes verifies handling of different panic types
func TestPanicRecoveryDifferentPanicTypes(t *testing.T) {
	log := logger.New("error")
	m := metrics.New()

	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{"String panic", "string panic"},
		{"Error panic", http.ErrAbortHandler},
		{"Int panic", 42},
		{"Nil panic", nil},
		{"Struct panic", struct{ msg string }{"panic"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := PanicRecovery(log, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(tt.panicValue)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			// Should not panic to test runner
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("Expected status 500, got %d", w.Code)
			}
		})
	}
}

// BenchmarkPanicRecovery measures panic recovery overhead (normal case)
func BenchmarkPanicRecovery(b *testing.B) {
	log := logger.New("error")
	m := metrics.New()

	handler := PanicRecovery(log, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// BenchmarkPanicRecoveryWithPanic measures panic recovery overhead (panic case)
func BenchmarkPanicRecoveryWithPanic(b *testing.B) {
	log := logger.New("error")
	m := metrics.New()

	handler := PanicRecovery(log, m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("benchmark panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
