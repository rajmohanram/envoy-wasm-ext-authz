// Package utils provides utility functions for secure logging and data masking.
// It implements JSON value masking and header sanitization to prevent
// sensitive data exposure in logs.
package utils

import (
	"encoding/json"
	"strings"
)

// MaskJSONValues masks all values in a JSON string for secure logging.
// It replaces actual values with "***" to prevent sensitive data leakage in logs.
//
// The function handles:
//   - Nested objects and arrays (recursively masked)
//   - All JSON primitive types (strings, numbers, booleans)
//   - null values (preserved as-is)
//   - Invalid JSON (returns error placeholder)
//
// Example:
//
//	input:  {"user":"alice","password":"secret123"}
//	output: {"user":"***","password":"***"}
//
// Performance: Optimized with minimal allocations for production use.
func MaskJSONValues(jsonStr string) string {
	if jsonStr == "" || jsonStr == "{}" {
		return jsonStr
	}

	// Fast path: Try to parse as JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// Not valid JSON, return masked placeholder
		return "{\"error\":\"invalid_json\"}"
	}

	// Recursively mask all values
	masked := maskMap(data)

	// Re-encode to JSON (this should never fail)
	result, _ := json.Marshal(masked)
	return string(result)
}

// maskMap recursively masks all values in a map
func maskMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = maskValue(v)
	}
	return result
}

// maskValue masks a single value based on its type
func maskValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		// Recursively mask nested objects
		return maskMap(val)
	case []any:
		// Recursively mask arrays
		masked := make([]any, len(val))
		for i, item := range val {
			masked[i] = maskValue(item)
		}
		return masked
	case string:
		// Mask strings, show length
		if len(val) == 0 {
			return ""
		}
		return "***"
	case int, int8, int16, int32, int64:
		// Mask numbers
		return "***"
	case float32, float64:
		// Mask floats
		return "***"
	case bool:
		// Mask booleans
		return "***"
	case nil:
		// Keep null as-is
		return nil
	default:
		// Unknown type, return masked
		return "***"
	}
}

// MaskHeaders masks sensitive HTTP header values for secure logging.
// Non-sensitive headers are preserved, while sensitive ones are replaced with "[MASKED]".
//
// Sensitive headers include:
//   - authorization
//   - cookie, set-cookie
//   - x-api-key, x-auth-token
//   - proxy-authorization
//
// Returns a map of header name to masked/unmasked value string.
// Multi-value headers are joined with commas.
func MaskHeaders(headers map[string][]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for name, values := range headers {
		// Show header name but mask value
		nameLower := strings.ToLower(name)
		if IsSensitiveHeader(nameLower) {
			masked[name] = "[MASKED]"
		} else {
			masked[name] = strings.Join(values, ",")
		}
	}
	return masked
}

// IsSensitiveHeader checks if an HTTP header name contains sensitive information
// that should be masked in logs.
//
// The check is case-sensitive and expects lowercase header names.
// Callers should use strings.ToLower() before calling this function.
//
// Example:
//
//	if IsSensitiveHeader(strings.ToLower("Authorization")) {
//	    // mask this header value
//	}
func IsSensitiveHeader(name string) bool {
	sensitiveHeaders := []string{
		"authorization",
		"cookie",
		"set-cookie",
		"x-api-key",
		"x-auth-token",
		"proxy-authorization",
	}

	for _, sensitive := range sensitiveHeaders {
		if name == sensitive {
			return true
		}
	}
	return false
}
