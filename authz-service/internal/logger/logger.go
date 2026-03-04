// Package logger provides structured logging with configurable log levels and async support.
//
// Features:
//   - Four log levels: DEBUG, INFO, WARN, ERROR
//   - Async logging via lock-free ring buffer (diode) for high-throughput scenarios
//   - Sync logging for development and testing
//   - Conditional logging to minimize overhead when log level filters messages
//   - Graceful log flushing on shutdown
//
// Environment Variables:
//   - LOG_ASYNC: Enable async logging (default: true)
//   - LOG_BUFFER_SIZE: Async buffer size 100-10000 (default: 5000)
//   - LOG_DROP_ON_OVERFLOW: Drop oldest logs when buffer full (default: true)
//
// The async mode uses zerolog's diode writer for non-blocking writes.
// Always call Close() before application exit to flush buffered logs.
//
// Example:
//
//	log := logger.New("info")
//	defer log.Close()
//	log.Info("Service starting on port %d", 8080)
package logger

import (
	"fmt"
	"github.com/rajmohanram/envoy-wasm-ext-authz/authz-service/internal/utils"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// Logger provides structured logging with configurable levels and async support.
// It wraps zerolog for efficient, structured logging with minimal allocations.
type Logger struct {
	level        LogLevel       // Minimum level to log (messages below this are discarded)
	logger       zerolog.Logger // Underlying zerolog instance
	diodeWriter  io.WriteCloser // Async ring buffer writer (nil in sync mode)
	asyncEnabled bool           // Whether async logging is active
}

// New creates a new logger with the specified level string.
//
// Valid level strings: "debug", "info", "warn", "warning", "error" (case-insensitive)
// Invalid levels default to "info".
//
// The logger reads async configuration from environment variables:
//   - LOG_ASYNC: Enable async mode (default: true)
//   - LOG_BUFFER_SIZE: Ring buffer size 100-10000 (default: 5000, clamped to bounds)
//   - LOG_DROP_ON_OVERFLOW: Drop oldest vs block on full buffer (default: true for non-blocking)
//
// Always defer Close() after creating a logger to ensure buffered logs are flushed:
//
//	log := logger.New("info")
//	defer log.Close()
func New(levelStr string) *Logger {
	level := parseLevel(levelStr)

	// Parse async configuration from environment
	asyncEnabled := utils.GetBool("LOG_ASYNC", true)
	bufferSize := utils.GetInt("LOG_BUFFER_SIZE", 5000)
	dropOnOverflow := utils.GetBool("LOG_DROP_ON_OVERFLOW", true)

	// Clamp buffer size to reasonable bounds
	if bufferSize < 100 {
		bufferSize = 100
	}
	if bufferSize > 10000 {
		bufferSize = 10000
	}

	var writer io.Writer
	var diodeWriter io.WriteCloser

	if asyncEnabled {
		// Create async diode writer with console output
		consoleWriter := createConsoleWriter(os.Stdout)

		// Create diode writer (lock-free ring buffer)
		if dropOnOverflow {
			// Drop oldest logs when buffer is full (recommended for production)
			diodeWriter = diode.NewWriter(consoleWriter, bufferSize, 0, func(missed int) {
				fmt.Fprintf(os.Stderr, "Logger dropped %d messages due to full buffer\n", missed)
			})
		} else {
			// Block when buffer is full (not recommended - can cause backpressure)
			diodeWriter = diode.NewWriter(consoleWriter, bufferSize, 10*time.Millisecond, func(missed int) {
				// Should never be called in blocking mode
			})
		}
		writer = diodeWriter
	} else {
		// Sync mode - direct console output
		consoleWriter := createConsoleWriter(os.Stdout)
		writer = consoleWriter
		diodeWriter = nil
	}

	// Create zerolog logger
	zlogger := zerolog.New(writer).With().Timestamp().Logger()

	// Set zerolog level
	switch level {
	case DebugLevel:
		zlogger = zlogger.Level(zerolog.DebugLevel)
	case InfoLevel:
		zlogger = zlogger.Level(zerolog.InfoLevel)
	case WarnLevel:
		zlogger = zlogger.Level(zerolog.WarnLevel)
	case ErrorLevel:
		zlogger = zlogger.Level(zerolog.ErrorLevel)
	}

	return &Logger{
		level:        level,
		logger:       zlogger,
		diodeWriter:  diodeWriter,
		asyncEnabled: asyncEnabled,
	}
}

// Close flushes any buffered logs and closes the async writer
// MUST be called before application exit to prevent log loss
// It is safe to call Close() multiple times
func (l *Logger) Close() error {
	if l.diodeWriter != nil {
		err := l.diodeWriter.Close()
		l.diodeWriter = nil // Prevent double close
		return err
	}
	return nil
}

// parseLevel converts string to LogLevel
func parseLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

// createConsoleWriter creates a zerolog console writer with standard formatting
func createConsoleWriter(out io.Writer) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        out,
		TimeFormat: "2006/01/02 15:04:05.000000",
		NoColor:    true, // Disable colors for production logs
		FormatLevel: func(i interface{}) string {
			if ll, ok := i.(string); ok {
				switch ll {
				case "debug":
					return "[DEBUG]"
				case "info":
					return "[INFO] "
				case "warn":
					return "[WARN] "
				case "error":
					return "[ERROR]"
				default:
					return "[" + strings.ToUpper(ll) + "]"
				}
			}
			return ""
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
		FormatFieldName: func(i interface{}) string {
			return ""
		},
		FormatFieldValue: func(i interface{}) string {
			return ""
		},
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...any) {
	if l.level <= DebugLevel {
		msg := fmt.Sprintf(format, v...)
		l.logger.Debug().Msg(msg)
	}
}

// IsDebugEnabled returns true if debug logging is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.level <= DebugLevel
}

// Info logs an info message
func (l *Logger) Info(format string, v ...any) {
	if l.level <= InfoLevel {
		msg := fmt.Sprintf(format, v...)
		l.logger.Info().Msg(msg)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...any) {
	if l.level <= WarnLevel {
		msg := fmt.Sprintf(format, v...)
		l.logger.Warn().Msg(msg)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...any) {
	if l.level <= ErrorLevel {
		msg := fmt.Sprintf(format, v...)
		l.logger.Error().Msg(msg)
	}
}

// Fatal logs an error and exits
func (l *Logger) Fatal(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Fatal().Msg(msg)
}

// SetOutput sets the output destination for the logger
// Note: This creates a new logger instance and does NOT support async mode
// Used primarily for testing
func (l *Logger) SetOutput(w io.Writer) {
	// Create a new zerolog logger with the provided writer
	// Disable async mode for testing
	consoleWriter := createConsoleWriter(w)

	l.logger = zerolog.New(consoleWriter).With().Timestamp().Logger()

	// Set level
	switch l.level {
	case DebugLevel:
		l.logger = l.logger.Level(zerolog.DebugLevel)
	case InfoLevel:
		l.logger = l.logger.Level(zerolog.InfoLevel)
	case WarnLevel:
		l.logger = l.logger.Level(zerolog.WarnLevel)
	case ErrorLevel:
		l.logger = l.logger.Level(zerolog.ErrorLevel)
	}

	// Close any existing diode writer
	if l.diodeWriter != nil {
		_ = l.diodeWriter.Close() // Ignore error on close during output redirection
		l.diodeWriter = nil
	}
	l.asyncEnabled = false
}
