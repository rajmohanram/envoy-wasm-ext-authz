// Package types defines common data structures used across the authorization service.
package types

// AuthzResponse represents the authorization decision
// Currently in allow-all mode (reverse proxy pattern)
type AuthzResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}
