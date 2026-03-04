package utils

import (
	"testing"
)

// TestMaskJSONValues verifies JSON value masking
func TestMaskJSONValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple object",
			input:    `{"user":"admin","password":"secret123"}`,
			expected: `{"password":"***","user":"***"}`,
		},
		{
			name:     "Nested object",
			input:    `{"user":"admin","profile":{"email":"test@example.com","age":30}}`,
			expected: `{"profile":{"age":"***","email":"***"},"user":"***"}`,
		},
		{
			name:     "Array",
			input:    `{"users":["alice","bob","charlie"]}`,
			expected: `{"users":["***","***","***"]}`,
		},
		{
			name:     "Mixed types",
			input:    `{"name":"test","count":42,"active":true,"balance":99.99}`,
			expected: `{"active":"***","balance":"***","count":"***","name":"***"}`,
		},
		{
			name:     "Null values",
			input:    `{"name":"test","email":null}`,
			expected: `{"email":null,"name":"***"}`,
		},
		{
			name:     "Empty object",
			input:    `{}`,
			expected: `{}`,
		},
		{
			name:     "Empty string",
			input:    ``,
			expected: ``,
		},
		{
			name:     "Invalid JSON",
			input:    `not valid json`,
			expected: `{"error":"invalid_json"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskJSONValues(tt.input)
			if result != tt.expected {
				t.Errorf("Expected: %s\nGot: %s", tt.expected, result)
			}
		})
	}
}

// TestIsSensitiveHeader verifies sensitive header detection
func TestIsSensitiveHeader(t *testing.T) {
	tests := []struct {
		header   string
		expected bool
	}{
		{"authorization", true},
		{"cookie", true},
		{"set-cookie", true},
		{"x-api-key", true},
		{"x-auth-token", true},
		{"proxy-authorization", true},
		{"content-type", false},
		{"user-agent", false},
		{"x-request-id", false},
		{"accept", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := IsSensitiveHeader(tt.header)
			if result != tt.expected {
				t.Errorf("Header %s: expected %v, got %v", tt.header, tt.expected, result)
			}
		})
	}
}

// TestMaskHeaders verifies header masking
func TestMaskHeaders(t *testing.T) {
	headers := map[string][]string{
		"Authorization": {"Bearer token123"},
		"Content-Type":  {"application/json"},
		"Cookie":        {"session=abc123"},
		"X-Request-ID":  {"req-123"},
		"X-API-Key":     {"secret-key"},
	}

	masked := MaskHeaders(headers)

	// Sensitive headers should be masked
	if masked["Authorization"] != "[MASKED]" {
		t.Errorf("Expected Authorization to be masked, got %s", masked["Authorization"])
	}
	if masked["Cookie"] != "[MASKED]" {
		t.Errorf("Expected Cookie to be masked, got %s", masked["Cookie"])
	}
	if masked["X-API-Key"] != "[MASKED]" {
		t.Errorf("Expected X-API-Key to be masked, got %s", masked["X-API-Key"])
	}

	// Non-sensitive headers should not be masked
	if masked["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type to be unmasked, got %s", masked["Content-Type"])
	}
	if masked["X-Request-ID"] != "req-123" {
		t.Errorf("Expected X-Request-ID to be unmasked, got %s", masked["X-Request-ID"])
	}
}

// TestMaskEmptyString verifies handling of empty strings
func TestMaskEmptyString(t *testing.T) {
	input := `{"name":"","email":""}`
	expected := `{"email":"","name":""}`

	result := MaskJSONValues(input)
	if result != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, result)
	}
}

// TestMaskComplexNesting verifies deep nesting
func TestMaskComplexNesting(t *testing.T) {
	input := `{
		"level1": {
			"level2": {
				"level3": {
					"secret": "deep-value"
				}
			}
		}
	}`

	result := MaskJSONValues(input)

	// Should contain masked value
	if !contains(result, "***") {
		t.Error("Expected masked values in deeply nested object")
	}
}

// TestMaskArrayOfObjects verifies array of objects masking
func TestMaskArrayOfObjects(t *testing.T) {
	input := `{"users":[{"name":"alice","email":"alice@example.com"},{"name":"bob","email":"bob@example.com"}]}`

	result := MaskJSONValues(input)

	// Should contain masked values
	if !contains(result, "***") {
		t.Error("Expected masked values in array of objects")
	}

	// Should not contain original values
	if contains(result, "alice") || contains(result, "bob") {
		t.Error("Original values should be masked")
	}
}

// BenchmarkMaskJSONValues measures masking performance
func BenchmarkMaskJSONValues(b *testing.B) {
	input := `{"user":"admin","password":"secret123","email":"test@example.com","age":30,"active":true}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MaskJSONValues(input)
	}
}

// BenchmarkIsSensitiveHeader measures header check performance
func BenchmarkIsSensitiveHeader(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsSensitiveHeader("authorization")
		IsSensitiveHeader("content-type")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
