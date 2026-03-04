package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLoggerLevels verifies log level filtering
func TestLoggerLevels(t *testing.T) {
	// Disable async for predictable testing
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	tests := []struct {
		name          string
		level         string
		shouldLogInfo bool
		shouldLogWarn bool
	}{
		{"Debug level", "debug", true, true},
		{"Info level", "info", true, true},
		{"Warn level", "warn", false, true},
		{"Error level", "error", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New(tt.level)
			defer func() { _ = log.Close() }()

			buf := &bytes.Buffer{}
			log.SetOutput(buf)

			// Try logging at different levels
			log.Info("info message")
			log.Warn("warn message")

			output := buf.String()

			// Check if info message was logged
			hasInfo := strings.Contains(output, "info message")
			if hasInfo != tt.shouldLogInfo {
				t.Errorf("Info logging: expected %v, got %v", tt.shouldLogInfo, hasInfo)
			}

			// Check if warn message was logged
			hasWarn := strings.Contains(output, "warn message")
			if hasWarn != tt.shouldLogWarn {
				t.Errorf("Warn logging: expected %v, got %v", tt.shouldLogWarn, hasWarn)
			}
		})
	}
}

// TestIsDebugEnabled verifies debug check
func TestIsDebugEnabled(t *testing.T) {
	// Disable async for predictable testing
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	tests := []struct {
		level    string
		expected bool
	}{
		{"debug", true},
		{"info", false},
		{"warn", false},
		{"error", false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			log := New(tt.level)
			defer func() { _ = log.Close() }()

			result := log.IsDebugEnabled()
			if result != tt.expected {
				t.Errorf("Level %s: expected IsDebugEnabled=%v, got %v", tt.level, tt.expected, result)
			}
		})
	}
}

// TestParseLevel verifies level parsing
func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DebugLevel},
		{"DEBUG", DebugLevel},
		{"info", InfoLevel},
		{"INFO", InfoLevel},
		{"warn", WarnLevel},
		{"warning", WarnLevel},
		{"error", ErrorLevel},
		{"ERROR", ErrorLevel},
		{"invalid", InfoLevel}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("Input %s: expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

// TestLoggerDebug verifies debug logging
func TestLoggerDebug(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("debug")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Debug("test debug message")

	output := buf.String()
	if !strings.Contains(output, "test debug message") {
		t.Error("Debug message not logged")
	}
	if !strings.Contains(output, "[DEBUG]") {
		t.Error("Debug prefix not found")
	}
}

// TestLoggerInfo verifies info logging
func TestLoggerInfo(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Info("test info message")

	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Error("Info message not logged")
	}
	if !strings.Contains(output, "[INFO]") {
		t.Error("Info prefix not found")
	}
}

// TestLoggerWarn verifies warn logging
func TestLoggerWarn(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("warn")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Warn("test warn message")

	output := buf.String()
	if !strings.Contains(output, "test warn message") {
		t.Error("Warn message not logged")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("Warn prefix not found")
	}
}

// TestLoggerError verifies error logging
func TestLoggerError(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("error")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Error("test error message")

	output := buf.String()
	if !strings.Contains(output, "test error message") {
		t.Error("Error message not logged")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("Error prefix not found")
	}
}

// TestLoggerFormatting verifies printf-style formatting
func TestLoggerFormatting(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Info("User %s logged in with ID %d", "alice", 123)

	output := buf.String()
	if !strings.Contains(output, "User alice logged in with ID 123") {
		t.Errorf("Formatting failed: %s", output)
	}
}

// TestLoggerFilteringAtInfoLevel verifies info level filters debug
func TestLoggerFilteringAtInfoLevel(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Debug("debug message")
	log.Info("info message")

	output := buf.String()

	// Debug should not appear
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should be filtered at info level")
	}

	// Info should appear
	if !strings.Contains(output, "info message") {
		t.Error("Info message should appear at info level")
	}
}

// TestLoggerClose verifies Close() method flushes logs
func TestLoggerClose(t *testing.T) {
	// Save original stdout to restore later
	oldStdout := os.Stdout

	// Redirect stdout to /dev/null to avoid conflict with coverage generation
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("Failed to open /dev/null: %v", err)
	}
	defer func() { _ = devNull.Close() }()
	defer func() { os.Stdout = oldStdout }()

	os.Stdout = devNull

	// Test async mode
	_ = os.Setenv("LOG_ASYNC", "true")
	_ = os.Setenv("LOG_BUFFER_SIZE", "100")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()
	defer func() { _ = os.Unsetenv("LOG_BUFFER_SIZE") }()

	log := New("info")

	// Write some logs
	for i := range 10 {
		log.Info("test message %d", i)
	}

	// Close should not error
	if err := log.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Give async writer time to fully flush
	time.Sleep(100 * time.Millisecond)

	// Calling Close() again should not error
	if err := log.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}

	// Restore stdout before test ends
	os.Stdout = oldStdout
}

// TestLoggerSync verifies sync mode works correctly
func TestLoggerSync(t *testing.T) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	log.Info("sync test message")

	output := buf.String()
	if !strings.Contains(output, "sync test message") {
		t.Error("Sync message not logged")
	}
}

// BenchmarkLoggerInfo measures info logging performance
func BenchmarkLoggerInfo(b *testing.B) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	b.ResetTimer()
	for range b.N {
		log.Info("benchmark message %d", b.N)
	}
}

// BenchmarkLoggerDebugFiltered measures filtered debug performance
func BenchmarkLoggerDebugFiltered(b *testing.B) {
	_ = os.Setenv("LOG_ASYNC", "false")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()

	log := New("info") // Debug will be filtered
	defer func() { _ = log.Close() }()

	buf := &bytes.Buffer{}
	log.SetOutput(buf)

	b.ResetTimer()
	for range b.N {
		log.Debug("benchmark debug message %d", b.N)
	}
}

// BenchmarkLoggerAsync measures async logging performance
func BenchmarkLoggerAsync(b *testing.B) {
	_ = os.Setenv("LOG_ASYNC", "true")
	_ = os.Setenv("LOG_BUFFER_SIZE", "10000")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()
	defer func() { _ = os.Unsetenv("LOG_BUFFER_SIZE") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	b.ResetTimer()
	for range b.N {
		log.Info("benchmark async message %d", b.N)
	}
	b.StopTimer()

	// Give time for async writer to flush
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkLoggerAsyncParallel measures async logging under concurrent load
func BenchmarkLoggerAsyncParallel(b *testing.B) {
	_ = os.Setenv("LOG_ASYNC", "true")
	_ = os.Setenv("LOG_BUFFER_SIZE", "10000")
	defer func() { _ = os.Unsetenv("LOG_ASYNC") }()
	defer func() { _ = os.Unsetenv("LOG_BUFFER_SIZE") }()

	log := New("info")
	defer func() { _ = log.Close() }()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			log.Info("parallel benchmark message %d", i)
			i++
		}
	})
	b.StopTimer()

	// Give time for async writer to flush
	time.Sleep(100 * time.Millisecond)
}
