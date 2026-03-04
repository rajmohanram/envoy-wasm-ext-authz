// Package config provides configuration management for the authorization service.
// Configuration is loaded from environment variables with sensible production defaults.
//
// Environment Variables:
//   - PORT: Main authorization server port - default: "443"
//   - WAF_PROXY_PORT: WAF proxy server port - default: "8443"
//   - LOG_LEVEL: Logging level (debug, info, warn, error) - default: "info"
//   - READ_TIMEOUT: HTTP read timeout in seconds - default: 10
//   - WRITE_TIMEOUT: HTTP write timeout in seconds - default: 10
//   - SHUTDOWN_TIMEOUT: Graceful shutdown timeout in seconds - default: 15
//   - ENABLE_METRICS: Enable /metrics endpoint - default: false
//   - FAIL_OPEN: Allow all requests when true, forward to WAF when false - default: true
//   - WAF_ENABLED: Enable WAF integration - default: false
//   - WAF_URL: WAF service URL - default: "https://waf-service:8443"
//   - WAF_TIMEOUT: WAF request timeout in seconds - default: 5
//   - WAF_TLS_SKIP_VERIFY: Skip TLS verification (insecure, use only in dev) - default: false
package config

import (
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/utils"
	"strings"
)

// Config holds all service configuration settings.
// All fields are loaded from environment variables with sensible defaults.
type Config struct {
	Port            string // Main authorization server port (default: "443")
	LogLevel        string // Logging level: debug, info, warn, error
	ReadTimeout     int    // HTTP server read timeout in seconds
	WriteTimeout    int    // HTTP server write timeout in seconds
	ShutdownTimeout int    // Graceful shutdown timeout in seconds
	EnableMetrics   bool   // Enable /metrics endpoint for performance monitoring
	FailOpen        bool   // Authorization mode: true=allow all, false=check with WAF
	// WAF Integration Settings
	WAFEnabled       bool   // Enable Web Application Firewall integration
	WAFURL           string // WAF service base URL
	WAFProxyPort     string // Port for WAF-inspected traffic (default: "8443")
	WAFTimeout       int    // WAF request timeout in seconds
	WAFTLSSkipVerify bool   // Skip TLS certificate verification (insecure, dev only)
}

// Load reads configuration from environment variables and returns a Config instance
// with production-safe defaults.
//
// This function never fails - invalid or missing environment variables fall back to
// sensible defaults to ensure the service can always start.
func Load() *Config {
	return &Config{
		Port:            utils.GetString("PORT", "443"), // Main authorization port
		LogLevel:        strings.ToLower(utils.GetString("LOG_LEVEL", "info")),
		ReadTimeout:     utils.GetInt("READ_TIMEOUT", 10),     // 10 seconds
		WriteTimeout:    utils.GetInt("WRITE_TIMEOUT", 10),    // 10 seconds
		ShutdownTimeout: utils.GetInt("SHUTDOWN_TIMEOUT", 15), // 15 seconds
		EnableMetrics:   utils.GetBool("ENABLE_METRICS", false),
		FailOpen:        utils.GetBool("FAIL_OPEN", true), // Default: true (allow-all mode)
		// WAF Integration
		WAFEnabled:       utils.GetBool("WAF_ENABLED", false), // Default: false (disabled)
		WAFURL:           utils.GetString("WAF_URL", "https://waf-service:8443"),
		WAFProxyPort:     utils.GetString("WAF_PROXY_PORT", "8443"), // WAF proxy port
		WAFTimeout:       utils.GetInt("WAF_TIMEOUT", 5),
		WAFTLSSkipVerify: utils.GetBool("WAF_TLS_SKIP_VERIFY", false), // Default: false (secure by default)
	}
}
