// Package handlers implements HTTP handlers for the authorization service.
// It provides handlers for authorization requests, health checks, metrics,
// descriptors, and WAF proxy endpoints.
package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/descriptors"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/metrics"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/types"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/utils"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/waf"
	"net/http"
	"strings"
	"time"
)

// AuthzHandler handles authorization requests (reverse proxy mode)
// Supports two modes based on fail_open configuration:
// - fail_open=true: Allow all requests immediately
// - fail_open=false: Forward to WAF for authorization
type AuthzHandler struct {
	logger     *logger.Logger
	metrics    *metrics.Metrics
	failOpen   bool // If true, allow all; if false, forward to WAF
	wafClient  *waf.Client
	wafEnabled bool
}

// NewAuthzHandler creates a new authorization handler
func NewAuthzHandler(log *logger.Logger, m *metrics.Metrics, failOpen bool, wafClient *waf.Client, wafEnabled bool) *AuthzHandler {
	return &AuthzHandler{
		logger:     log,
		metrics:    m,
		failOpen:   failOpen,
		wafClient:  wafClient,
		wafEnabled: wafEnabled,
	}
}

// ServeHTTP implements http.Handler interface for authorization requests.
// It processes requests in three steps:
// 1. Logs the incoming request with source IP information
// 2. Makes authorization decision based on configuration (fail-open mode or WAF check)
// 3. Records metrics and returns JSON response
//
// Authorization modes:
//   - fail_open=true: Immediately allows all requests (development/testing)
//   - fail_open=false + WAF enabled: Forwards to WAF for decision
//   - fail_open=false + WAF disabled: Denies all requests by default
func (h *AuthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Log request with client IP chain (supports X-Forwarded-For)
	h.logRequest(r)

	// Only log headers when DEBUG logging is enabled to minimize overhead
	if h.logger.IsDebugEnabled() {
		h.logHeaders(r)
	}

	// Make authorization decision
	allowed, reason := h.authorize(r)

	// Send response and record metrics
	h.sendAuthzResponse(w, allowed, reason)
	h.recordMetrics(allowed, time.Since(start))

	// Log final decision
	decision := "allowed"
	if !allowed {
		decision = "denied"
	}
	h.logger.Info("Request %s %s processed in %v (%s)", r.Method, r.URL.Path, time.Since(start), decision)
}

// logRequest logs the incoming request with source IP information
func (h *AuthzHandler) logRequest(r *http.Request) {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		h.logger.Info("AUTHZ REQUEST: %s %s from %s (XFF: %s)", r.Method, r.URL.Path, r.RemoteAddr, xff)
	} else {
		h.logger.Info("AUTHZ REQUEST: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	}
}

// authorize makes the authorization decision based on configuration
func (h *AuthzHandler) authorize(r *http.Request) (allowed bool, reason string) {
	if h.failOpen {
		return true, "Allow-all mode (fail_open=true)"
	}

	if !h.wafEnabled {
		h.logger.Warn("WAF not enabled but fail_open=false - denying request by default")
		return false, "WAF not enabled (fail_open=false)"
	}

	return h.callWAF(r)
}

// recordMetrics updates metrics based on the authorization result
func (h *AuthzHandler) recordMetrics(allowed bool, duration time.Duration) {
	if allowed {
		h.metrics.IncAllowed()
	} else {
		h.metrics.IncDenied()
	}
	h.metrics.RecordLatency(duration)
}

// logHeaders logs request headers with sensitive values masked
func (h *AuthzHandler) logHeaders(r *http.Request) {
	h.logger.Debug("Request Headers:")

	// Log HTTP/2 pseudo-header equivalents (Go converts these from HTTP/2 frame)
	h.logger.Debug("  :authority: %s", r.Host)
	h.logger.Debug("  :method: %s", r.Method)
	h.logger.Debug("  :path: %s", r.URL.Path)
	if r.URL.Scheme != "" {
		h.logger.Debug("  :scheme: %s", r.URL.Scheme)
	}
	h.logger.Debug("  ---")

	// Log regular headers
	for name, values := range r.Header {
		// Mask sensitive headers
		nameLower := strings.ToLower(name)
		if utils.IsSensitiveHeader(nameLower) {
			h.logger.Debug("  %s: [MASKED]", name)
		} else {
			for _, value := range values {
				h.logger.Debug("  %s: %s", name, value)
			}
		}
	}
}

// sendAuthzResponse sends authorization response
func (h *AuthzHandler) sendAuthzResponse(w http.ResponseWriter, allowed bool, reason string) {
	decision := types.AuthzResponse{
		Allowed: allowed,
		Reason:  reason,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(decision); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
		h.metrics.IncErrors()
	}
}

// callWAF forwards the request to WAF and makes authorization decision
// Uses streaming reverse proxy for optimal performance (zero-copy) in all scenarios
func (h *AuthzHandler) callWAF(r *http.Request) (bool, string) {
	// Add AUTHZ service's client IP to X-Forwarded-For chain
	// This ensures WAF knows the full proxy chain
	h.appendClientIPToXFF(r)

	h.logger.Info("Forwarding request to WAF: %s %s", r.Method, r.URL.Path)

	// Create a custom response writer to capture the status code
	captureWriter := &statusCapturingWriter{
		ResponseWriter: h.createDummyResponseWriter(),
		statusCode:     200, // Default
	}

	// Stream to WAF using reverse proxy (zero-copy, maximum performance)
	err := h.wafClient.StreamRequest(captureWriter, r)
	if err != nil {
		h.logger.Error("WAF streaming call failed: %v", err)
		if h.failOpen {
			return true, "WAF unavailable (fail-open)"
		}
		return false, "WAF unavailable (fail-closed)"
	}

	// Decision based on captured status code
	allowed := waf.IsAllowed(captureWriter.statusCode)
	reason := fmt.Sprintf("WAF decision: %d", captureWriter.statusCode)

	if allowed {
		h.logger.Info("WAF allowed request: status=%d", captureWriter.statusCode)
	} else {
		h.logger.Warn("WAF denied request: status=%d", captureWriter.statusCode)
	}

	return allowed, reason
}

// appendClientIPToXFF adds the client IP from RemoteAddr to X-Forwarded-For header
// This maintains the proxy chain: client -> envoy -> authz -> waf
func (h *AuthzHandler) appendClientIPToXFF(r *http.Request) {
	// Extract IP from RemoteAddr (format: "ip:port")
	clientIP := r.RemoteAddr
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx] // Remove port
	}

	// Append to X-Forwarded-For chain
	if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
		r.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		r.Header.Set("X-Forwarded-For", clientIP)
	}

	h.logger.Debug("Updated X-Forwarded-For: %s", r.Header.Get("X-Forwarded-For"))
}

// statusCapturingWriter captures the HTTP status code from WAF response
type statusCapturingWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCapturingWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

// createDummyResponseWriter creates a dummy response writer for capturing WAF response
func (h *AuthzHandler) createDummyResponseWriter() http.ResponseWriter {
	// Create a minimal response writer that discards the body
	return &dummyResponseWriter{
		header: make(http.Header),
	}
}

type dummyResponseWriter struct {
	header http.Header
}

func (w *dummyResponseWriter) Header() http.Header {
	return w.header
}

func (w *dummyResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil // Discard body
}

func (w *dummyResponseWriter) WriteHeader(statusCode int) {
	// No-op
}

// sendJSONResponse is a shared utility for sending JSON responses
func sendJSONResponse(w http.ResponseWriter, logger *logger.Logger, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode response: %v", err)
	}
}

// HealthHandler handles health check requests
type HealthHandler struct {
	logger *logger.Logger
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(log *logger.Logger) *HealthHandler {
	return &HealthHandler{
		logger: log,
	}
}

// ServeHTTP implements http.Handler
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sendJSONResponse(w, h.logger, http.StatusOK, map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "grpc-authz-service",
	})
}

// MetricsHandler handles metrics requests
type MetricsHandler struct {
	metrics *metrics.Metrics
	logger  *logger.Logger
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(m *metrics.Metrics, log *logger.Logger) *MetricsHandler {
	return &MetricsHandler{
		metrics: m,
		logger:  log,
	}
}

// ServeHTTP implements http.Handler
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stats := h.metrics.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Error("Failed to encode metrics response: %v", err)
	}
}

// DescriptorHandler serves proto descriptors
type DescriptorHandler struct {
	store  *descriptors.Store
	logger *logger.Logger
}

// NewDescriptorHandler creates a new descriptor handler
func NewDescriptorHandler(store *descriptors.Store, log *logger.Logger) *DescriptorHandler {
	return &DescriptorHandler{
		store:  store,
		logger: log,
	}
}

// ServeHTTP implements http.Handler
// Supports:
//
//	GET /descriptors           - List all descriptors
//	GET /descriptors/{name}    - Get specific descriptor (binary)
//	GET /descriptors/{name}?format=base64 - Get descriptor as base64
func (h *DescriptorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/descriptors")
	path = strings.TrimPrefix(path, "/")

	// List all descriptors
	if path == "" {
		h.listDescriptors(w)
		return
	}

	// Get specific descriptor
	h.getDescriptor(w, r, path)
}

// listDescriptors returns all available descriptors
func (h *DescriptorHandler) listDescriptors(w http.ResponseWriter) {
	names := h.store.List()

	response := map[string]any{
		"descriptors": names,
		"count":       len(names),
	}

	sendJSONResponse(w, h.logger, http.StatusOK, response)
	h.logger.Debug("Served descriptor list (%d descriptors)", len(names))
}

// getDescriptor returns a specific descriptor
func (h *DescriptorHandler) getDescriptor(w http.ResponseWriter, r *http.Request, name string) {
	data, err := h.store.Get(name)
	if err != nil {
		h.logger.Warn("Descriptor not found: %s", name)
		http.Error(w, "Descriptor not found", http.StatusNotFound)
		return
	}

	// Check format
	format := r.URL.Query().Get("format")

	if format == "base64" {
		h.sendDescriptorAsBase64(w, name, data)
	} else {
		h.sendDescriptorAsBinary(w, name, data)
	}
}

// sendDescriptorAsBase64 sends descriptor as base64-encoded JSON
func (h *DescriptorHandler) sendDescriptorAsBase64(w http.ResponseWriter, name string, data []byte) {
	response := map[string]any{
		"name":       name,
		"data":       base64.StdEncoding.EncodeToString(data),
		"size_bytes": len(data),
	}

	sendJSONResponse(w, h.logger, http.StatusOK, response)
	h.logger.Debug("Served descriptor '%s' as base64 (%d bytes)", name, len(data))
}

// sendDescriptorAsBinary sends descriptor as binary data
func (h *DescriptorHandler) sendDescriptorAsBinary(w http.ResponseWriter, name string, data []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(data); err != nil {
		h.logger.Error("Failed to write descriptor: %v", err)
	}

	h.logger.Debug("Served descriptor '%s' as binary (%d bytes)", name, len(data))
}
