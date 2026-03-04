// Package middleware provides HTTP middleware components for the authorization service.
//
// Following the KISS (Keep It Simple, Stupid) principle, this package implements
// only panic recovery middleware. Other common middleware concerns like rate limiting,
// request IDs, and access logging are deliberately delegated to Envoy proxy for:
//   - Reduced complexity in the application layer
//   - Centralized infrastructure concerns
//   - Better separation of responsibilities
//   - Improved performance (handled at proxy layer)
//
// The panic recovery middleware ensures the service remains available even when
// unexpected panics occur, logging full stack traces for debugging while returning
// safe error responses to clients.
package middleware

import (
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/metrics"
	"net/http"
	"runtime/debug"
)

// PanicRecovery returns a middleware handler that recovers from panics in HTTP handlers.
// When a panic occurs:
//  1. The panic value and full stack trace are logged at ERROR level
//  2. The error counter is incremented in metrics
//  3. A generic 500 Internal Server Error is returned to the client
//
// This prevents service crashes while avoiding exposure of internal details to clients.
// The middleware is thread-safe and can be used with concurrent request handlers.
//
// Example usage:
//
//	handler := middleware.PanicRecovery(log, metrics)(myHandler)
//	http.ListenAndServe(":8080", handler)
func PanicRecovery(log *logger.Logger, m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					log.Error("PANIC RECOVERED: %v", err)
					log.Error("Stack trace:\n%s", string(debug.Stack()))

					// Increment error metrics
					m.IncErrors()

					// Return 500 error to client (don't expose internal details)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
