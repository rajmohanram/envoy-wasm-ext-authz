package utils

import (
	"os"
	"testing"
)

func TestGetString(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultVal string
		envVal     string
		setEnv     bool
		want       string
	}{
		{
			name:       "returns default when env var not set",
			key:        "TEST_STRING",
			defaultVal: "default",
			setEnv:     false,
			want:       "default",
		},
		{
			name:       "returns env var when set",
			key:        "TEST_STRING",
			defaultVal: "default",
			envVal:     "from-env",
			setEnv:     true,
			want:       "from-env",
		},
		{
			name:       "returns empty string from env when set to empty",
			key:        "TEST_STRING",
			defaultVal: "default",
			envVal:     "",
			setEnv:     true,
			want:       "default", // Empty string means not set, so default is returned
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			_ = os.Unsetenv(tt.key)
			defer func() { _ = os.Unsetenv(tt.key) }()

			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envVal)
			}

			got := GetString(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultVal int
		envVal     string
		setEnv     bool
		want       int
	}{
		{
			name:       "returns default when env var not set",
			key:        "TEST_INT",
			defaultVal: 42,
			setEnv:     false,
			want:       42,
		},
		{
			name:       "returns env var when set to valid int",
			key:        "TEST_INT",
			defaultVal: 42,
			envVal:     "100",
			setEnv:     true,
			want:       100,
		},
		{
			name:       "returns default when env var is invalid int",
			key:        "TEST_INT",
			defaultVal: 42,
			envVal:     "not-a-number",
			setEnv:     true,
			want:       42,
		},
		{
			name:       "handles negative numbers",
			key:        "TEST_INT",
			defaultVal: 42,
			envVal:     "-10",
			setEnv:     true,
			want:       -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			_ = os.Unsetenv(tt.key)
			defer func() { _ = os.Unsetenv(tt.key) }()

			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envVal)
			}

			got := GetInt(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultVal bool
		envVal     string
		setEnv     bool
		want       bool
	}{
		{
			name:       "returns default when env var not set",
			key:        "TEST_BOOL",
			defaultVal: true,
			setEnv:     false,
			want:       true,
		},
		{
			name:       "returns true when env var is 'true'",
			key:        "TEST_BOOL",
			defaultVal: false,
			envVal:     "true",
			setEnv:     true,
			want:       true,
		},
		{
			name:       "returns false when env var is 'false'",
			key:        "TEST_BOOL",
			defaultVal: true,
			envVal:     "false",
			setEnv:     true,
			want:       false,
		},
		{
			name:       "returns true when env var is '1'",
			key:        "TEST_BOOL",
			defaultVal: false,
			envVal:     "1",
			setEnv:     true,
			want:       true,
		},
		{
			name:       "returns false when env var is '0'",
			key:        "TEST_BOOL",
			defaultVal: true,
			envVal:     "0",
			setEnv:     true,
			want:       false,
		},
		{
			name:       "returns default when env var is invalid",
			key:        "TEST_BOOL",
			defaultVal: true,
			envVal:     "not-a-bool",
			setEnv:     true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			_ = os.Unsetenv(tt.key)
			defer func() { _ = os.Unsetenv(tt.key) }()

			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envVal)
			}

			got := GetBool(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}
