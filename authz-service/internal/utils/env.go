// Package utils provides utility functions for the authorization service.
// It includes environment variable parsing, JSON value masking for secure logging,
// and header sanitization to prevent sensitive data exposure.
package utils

import (
	"os"
	"strconv"
)

// GetString reads a string environment variable with a default fallback value.
// Returns the environment variable value if set and non-empty, otherwise returns the default.
//
// Example:
//
//	logLevel := utils.GetString("LOG_LEVEL", "info")  // Returns "info" if LOG_LEVEL not set
func GetString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GetInt reads an integer environment variable with a default fallback value.
// Returns the parsed integer if the environment variable is set and valid,
// otherwise returns the default value.
//
// Invalid integers (non-numeric strings) are treated as unset and return the default.
//
// Example:
//
//	timeout := utils.GetInt("TIMEOUT", 30)  // Returns 30 if TIMEOUT not set or invalid
func GetInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetBool reads a boolean environment variable with a default fallback value.
// Returns the parsed boolean if the environment variable is set and valid,
// otherwise returns the default value.
//
// Valid boolean strings: "1", "t", "T", "true", "TRUE", "True", "0", "f", "F", "false", "FALSE", "False"
// Invalid values are treated as unset and return the default.
//
// Example:
//
//	debug := utils.GetBool("DEBUG", false)  // Returns false if DEBUG not set or invalid
func GetBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
