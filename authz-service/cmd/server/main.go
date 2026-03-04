package main

import (
	"context"
	"fmt"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/config"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/descriptors"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/handlers"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/metrics"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/middleware"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/waf"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log := logger.New(cfg.LogLevel)
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()
	log.Info("=== gRPC Authorization Service (Reverse Proxy Mode) ===")
	log.Info("Version: 3.0.0")

	// Log fail_open mode
	if cfg.FailOpen {
		log.Info("Mode: FAIL_OPEN (allow all requests)")
		log.Info("Description: All authorization requests will be ALLOWED immediately")
	} else {
		log.Info("Mode: FAIL_CLOSED (forward to WAF)")
		if cfg.WAFEnabled {
			log.Info("WAF integration enabled - requests will be forwarded for authorization")
		} else {
			log.Warn("WARNING: WAF not enabled - requests will be DENIED by default")
		}
	}

	log.Info("Log Level: %s", cfg.LogLevel)

	// Initialize metrics
	m := metrics.New()
	if cfg.EnableMetrics {
		log.Info("Metrics enabled on /metrics endpoint")
	}

	// Initialize WAF client if enabled
	var wafClient *waf.Client
	if cfg.WAFEnabled {
		wafClient = waf.NewClient(
			cfg.WAFURL,
			time.Duration(cfg.WAFTimeout)*time.Second,
			cfg.WAFTLSSkipVerify,
			log,
		)
		log.Info("WAF integration enabled: %s (timeout: %ds)", cfg.WAFURL, cfg.WAFTimeout)
	} else {
		log.Info("WAF integration disabled")
	}

	// Initialize descriptor store
	descriptorStore := descriptors.NewStore(log)

	// Load descriptor from file (MANDATORY)
	descriptorFile := os.Getenv("DESCRIPTOR_FILE")
	if descriptorFile == "" {
		log.Error("DESCRIPTOR_FILE environment variable is required but not set")
		log.Error("Please set DESCRIPTOR_FILE to the path of your proto descriptor file")
		os.Exit(1)
	}

	// Extract the base filename from the path to use as the descriptor name
	descriptorName := filepath.Base(descriptorFile)
	if err := descriptorStore.LoadFromFile(descriptorName, descriptorFile); err != nil {
		log.Error("Failed to load descriptor from file %s: %v", descriptorFile, err)
		log.Error("Cannot start without valid descriptor file")
		os.Exit(1)
	}

	log.Info("Descriptor '%s' loaded from: %s", descriptorName, descriptorFile)
	log.Info("Descriptors loaded: %d", descriptorStore.Count())

	// Initialize handlers
	authzHandler := handlers.NewAuthzHandler(log, m, cfg.FailOpen, wafClient, cfg.WAFEnabled)
	healthHandler := handlers.NewHealthHandler(log)
	descriptorHandler := handlers.NewDescriptorHandler(descriptorStore, log)
	wafProxyHandler := handlers.NewWafProxyHandler(log)

	// Setup HTTP server with reverse proxy pattern
	mux := http.NewServeMux()

	// Utility endpoints (fixed paths)
	mux.Handle("/health", healthHandler)
	mux.Handle("/descriptors", descriptorHandler)
	mux.Handle("/descriptors/", descriptorHandler)

	if cfg.EnableMetrics {
		metricsHandler := handlers.NewMetricsHandler(m, log)
		mux.Handle("/metrics", metricsHandler)
	}

	// Authorization endpoint - accept ALL paths (reverse proxy mode)
	// This catch-all must be registered last to not override specific handlers above
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Skip utility endpoints that are already registered
		if r.URL.Path == "/health" ||
			r.URL.Path == "/metrics" ||
			strings.HasPrefix(r.URL.Path, "/descriptors") {
			http.NotFound(w, r)
			return
		}

		// All other paths go to authz handler
		authzHandler.ServeHTTP(w, r)
	})

	// Apply middleware: Only panic recovery (KISS principle)
	// Rate limiting, logging, and request IDs are handled by Envoy
	var handler http.Handler = mux
	handler = middleware.PanicRecovery(log, m)(handler)

	log.Info("Middleware enabled: panic-recovery only (KISS - other concerns handled by Envoy)")

	// Get TLS configuration from environment variables
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	// Setup primary HTTP server (port 443 - main authorization)
	server := createHTTPServer(":"+cfg.Port, handler, cfg)

	// Setup WAF proxy server (port 8443 - WAF-inspected traffic)
	wafProxyMux := http.NewServeMux()
	wafProxyMux.Handle("/health", healthHandler)
	wafProxyMux.Handle("/", wafProxyHandler)

	var wafProxyHandlerWrapped http.Handler = wafProxyMux
	wafProxyHandlerWrapped = middleware.PanicRecovery(log, m)(wafProxyHandlerWrapped)

	wafProxyServer := createHTTPServer(":"+cfg.WAFProxyPort, wafProxyHandlerWrapped, cfg)

	// Start server in a goroutine
	go func() {
		log.Info("Main server starting on port %s", cfg.Port)
		log.Info("Endpoints:")
		log.Info("  - ANY   /*               (authorization - WASM calls this)")
		log.Info("  - GET   /health          (health check)")
		log.Info("  - GET   /descriptors     (list proto descriptors)")
		log.Info("  - GET   /descriptors/:id (get specific descriptor)")
		if cfg.EnableMetrics {
			log.Info("  - GET   /metrics         (performance metrics)")
		}

		var err error
		if certFile != "" && keyFile != "" {
			log.Info("TLS enabled with HTTP/2 support (cert: %s, key: %s)", certFile, keyFile)
			err = server.ListenAndServeTLS(certFile, keyFile)
		} else {
			log.Info("Running without TLS (plain HTTP/1.1)")
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start main server: %v", err)
		}
	}()

	// Start WAF proxy server in a goroutine
	go func() {
		log.Info("WAF proxy server starting on port %s", cfg.WAFProxyPort)
		log.Info("Endpoints:")
		log.Info("  - GET   /health          (health check)")
		log.Info("  - ANY   /*               (WAF proxy - accepts traffic from WAF with X-WAF-Inspection header)")

		var err error
		if certFile != "" && keyFile != "" {
			err = wafProxyServer.ListenAndServeTLS(certFile, keyFile)
		} else {
			err = wafProxyServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start WAF proxy server: %v", err)
		}
	}()

	log.Info("Server started successfully")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Servers shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeout)*time.Second)
	defer cancel()

	// Shutdown both servers
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Main server forced to shutdown: %v", err)
	} else {
		log.Info("Main server stopped gracefully")
	}

	if err := wafProxyServer.Shutdown(ctx); err != nil {
		log.Error("WAF proxy server forced to shutdown: %v", err)
	} else {
		log.Info("WAF proxy server stopped gracefully")
	}

	// Print final metrics
	if cfg.EnableMetrics {
		stats := m.GetStats()
		log.Info("Final Statistics:")
		log.Info("  Total Requests: %d", stats.TotalRequests)
		log.Info("  Allowed: %d (%.1f%%)", stats.AllowedCount, stats.AllowRate)
		log.Info("  Denied: %d (%.1f%%)", stats.DeniedCount, stats.DenyRate)
		log.Info("  Errors: %d", stats.ErrorCount)
		log.Info("  Avg Latency: %.2fms", stats.AvgLatencyMs)
		log.Info("  Uptime: %ds", stats.UptimeSeconds)
	}

	log.Info("Shutdown complete")

	// Flush logger before exit to ensure all logs are written
	if err := log.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush logs: %v\n", err)
	}
}

// createHTTPServer creates an HTTP server with common configuration
func createHTTPServer(addr string, handler http.Handler, cfg *config.Config) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
