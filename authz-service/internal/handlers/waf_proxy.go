// Package handlers implements HTTP handlers for the authorization service.
// It provides handlers for authorization requests, health checks, metrics,
// descriptors, and WAF proxy endpoints.
package handlers

import (
	"encoding/json"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"net/http"
	"strings"
	"time"
)

// WafProxyHandler handles requests proxied from WAF (port 8443)
// This handler immediately returns 200 OK without any processing
type WafProxyHandler struct {
	logger *logger.Logger
}

// NewWafProxyHandler creates a new WAF proxy handler
func NewWafProxyHandler(log *logger.Logger) *WafProxyHandler {
	return &WafProxyHandler{
		logger: log,
	}
}

// ServeHTTP implements http.Handler
// Always returns HTTP 200 OK immediately
func (h *WafProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify X-WAF-Inspection header
	wafInspection := r.Header.Get("X-WAF-Inspection")
	if wafInspection != "true" {
		h.logger.Warn("[WAF-PROXY: Port 8443] Request without X-WAF-Inspection header from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized: Missing WAF inspection header", http.StatusUnauthorized)
		return
	}

	// Extract the actual source IP from X-Forwarded-For chain
	// The last IP in XFF is the immediate upstream (WAF service)
	sourceIP := r.RemoteAddr
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Parse XFF: "ip1, ip2, ip3" -> get last IP (rightmost = immediate upstream)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			// Get last IP and trim whitespace
			sourceIP = strings.TrimSpace(ips[len(ips)-1])
		}
	}

	// Log the proxied request with actual source IP
	h.logger.Info("[WAF-PROXY: Port 8443] WAF-proxied request: %s %s from %s", r.Method, r.URL.Path, sourceIP)
	h.logger.Debug("[WAF-PROXY: Port 8443] WAF inspection header present: %s", wafInspection)

	// Immediately return 200 OK with simple response
	response := map[string]any{
		"status":    "ok",
		"message":   "Request acknowledged by AUTHZ",
		"timestamp": time.Now().Unix(),
		"port":      8443,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("[WAF-PROXY: Port 8443] Failed to encode response: %v", err)
	}

	h.logger.Debug("[WAF-PROXY: Port 8443] Returned 200 OK to WAF")
}
