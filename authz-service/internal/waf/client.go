// Package waf provides HTTP client functionality for communicating with
// Web Application Firewall (WAF) services. It supports both streaming
// (zero-copy) and buffered request forwarding modes for optimal performance.
package waf

import (
	"crypto/tls"
	"fmt"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/logger"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// Client represents a WAF HTTP client
type Client struct {
	httpClient   *http.Client
	reverseProxy *httputil.ReverseProxy
	wafURL       string
	wafParsedURL *url.URL
	timeout      time.Duration
	logger       *logger.Logger
}

// NewClient creates a new WAF client with HTTP/2 support and reverse proxy capabilities.
// The client handles transparent request forwarding to the WAF service while preserving
// the original client's Host header for proper routing.
//
// Parameters:
//   - wafURL: The base URL of the WAF service (e.g., "https://waf-service:8443")
//   - timeout: Request timeout duration
//   - skipVerify: Whether to skip TLS certificate verification (use only in development)
//   - log: Logger instance for debug logging
//
// The client supports two modes:
//   - Streaming mode: Zero-copy request forwarding via reverse proxy (optimal performance)
//   - Buffered mode: Reads request body into memory for debug logging
func NewClient(wafURL string, timeout time.Duration, skipVerify bool, log *logger.Logger) *Client {
	transport := createTransport(skipVerify)
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// Parse WAF URL for reverse proxy setup
	parsedURL, err := url.Parse(wafURL)
	if err != nil {
		// Fallback to basic client if URL parsing fails
		log.Debug("Failed to parse WAF URL: %v", err)
		return &Client{
			httpClient: httpClient,
			wafURL:     wafURL,
			timeout:    timeout,
			logger:     log,
		}
	}

	// Create reverse proxy with custom rewrite for Host header preservation
	reverseProxy := httputil.NewSingleHostReverseProxy(parsedURL)
	reverseProxy.Transport = transport
	reverseProxy.Rewrite = createRewrite(log)

	return &Client{
		httpClient:   httpClient,
		reverseProxy: reverseProxy,
		wafURL:       wafURL,
		wafParsedURL: parsedURL,
		timeout:      timeout,
		logger:       log,
	}
}

// createTransport creates an HTTP transport with optimized connection pooling
func createTransport(skipVerify bool) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipVerify,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
}

// createRewrite creates a reverse proxy rewrite function that preserves the client's
// original Host header. This is critical for proper request routing through the WAF.
//
// The rewrite function performs these steps:
// 1. Captures the client's original :authority pseudo-header (HTTP/2) or Host header
// 2. Sets the target URL (same as default director)
// 3. Restores the client's original Host header
// 4. Cleans up HTTP/2 pseudo-headers for HTTP/1.1 compatibility
func createRewrite(log *logger.Logger) func(*httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		// Step 1: Capture client's original authority from incoming request
		clientAuthority := extractClientAuthority(pr.In)
		log.Debug("Captured authority: '%s', req.Host: '%s'", clientAuthority, pr.In.Host)

		// Step 2: SetURL sets the outgoing request URL (equivalent to default director)
		// This is already done by NewSingleHostReverseProxy
		log.Debug("After rewrite - req.Host: '%s', req.URL.Host: '%s'", pr.Out.Host, pr.Out.URL.Host)

		// Step 3: Restore client's original Host header in outgoing request
		if clientAuthority != "" {
			pr.Out.Host = clientAuthority
			pr.Out.Header.Set("Host", clientAuthority)
			log.Debug("Restored Host to: '%s'", clientAuthority)
		}

		// Step 4: Add internal marker and clean up pseudo-headers
		pr.Out.Header.Set("x-internal-authz", "true")
		prepareHeadersForWAF(pr.Out)
	}
}

// extractClientAuthority retrieves the client's original host/authority from the request.
// In HTTP/2, this is the :authority pseudo-header; in HTTP/1.1, it's the Host header.
func extractClientAuthority(req *http.Request) string {
	if authority := req.Header.Get(":authority"); authority != "" {
		return authority
	}
	return req.Host
}

// StreamRequest streams the request to WAF using reverse proxy (no body reading)
// This is used when debug logging is disabled for maximum performance
func (c *Client) StreamRequest(w http.ResponseWriter, r *http.Request) error {
	if c.reverseProxy == nil {
		return fmt.Errorf("reverse proxy not available")
	}

	// Stream to WAF using reverse proxy
	// Note: prepareHeadersForWAF is called by the Rewrite function, not here
	c.reverseProxy.ServeHTTP(w, r)
	return nil
}

// ForwardRequest forwards an authorization request to the WAF (reads body into memory)
// This is used when debug logging is enabled and body needs to be logged
// Returns the WAF response status code, body, and any error
func (c *Client) ForwardRequest(method, path string, headers http.Header, body []byte) (int, []byte, error) {
	// Build full URL
	fullURL := fmt.Sprintf("%s%s", c.wafURL, path)

	// CRITICAL: Extract :authority from incoming headers BEFORE creating the request
	// because http.NewRequest will set req.Host to the URL's host (waf-service:8443)
	clientAuthority := headers.Get(":authority")

	c.logger.Debug("[ForwardRequest] Incoming :authority from headers: '%s'", clientAuthority)

	// Create request with body
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create WAF request: %w", err)
	}

	c.logger.Debug("[ForwardRequest] After NewRequest - req.Host: '%s'", req.Host)

	// Forward all headers (preserving multi-value headers)
	req.Header = headers.Clone()

	// CRITICAL: Map :authority pseudo-header to Host header (HTTP/2 to HTTP/1.1 conversion)
	// Use the clientAuthority we extracted earlier (client's original host)
	if clientAuthority != "" {
		// Set both req.Host and Host header to the client's original authority
		req.Host = clientAuthority
		req.Header.Set("Host", clientAuthority)
		c.logger.Debug("[ForwardRequest] Set Host header to: %s (from :authority)", clientAuthority)
	} else {
		// Fallback: keep the default req.Host from URL
		req.Header.Set("Host", req.Host)
		c.logger.Debug("[ForwardRequest] WARNING: No :authority found, using URL host: %s", req.Host)
	}

	// Add internal authorization marker to filter out WAF proxy traffic in Envoy logs
	req.Header.Set("x-internal-authz", "true")

	// Clean up pseudo-headers (must be done AFTER capturing authority)
	prepareHeadersForWAF(req)

	// Log final Host header
	c.logger.Debug("[ForwardRequest] Final Host header: %s, req.Host: %s", req.Header.Get("Host"), req.Host)

	// Make request to WAF
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to call WAF: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override the main return error
			_ = closeErr
		}
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read WAF response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

// prepareHeadersForWAF removes HTTP/2 pseudo-headers for HTTP/1.1 compatibility
// The Host header mapping from :authority is handled by the Rewrite/ForwardRequest functions
func prepareHeadersForWAF(req *http.Request) {
	// Remove HTTP/2 pseudo-headers (not valid in HTTP/1.1 to WAF)
	// Note: :authority is already mapped to Host header by Director/ForwardRequest
	req.Header.Del(":authority")
	req.Header.Del(":method")
	req.Header.Del(":path")
	req.Header.Del(":scheme")
	req.Header.Del(":status")
}

// IsAllowed determines if a WAF response indicates the request is allowed
// 2xx and 3xx responses are considered "allowed"
// 4xx and 5xx responses are considered "denied"
func IsAllowed(statusCode int) bool {
	return statusCode >= 200 && statusCode < 400
}
