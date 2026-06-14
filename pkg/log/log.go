package log

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func init() {
	// Check for dev mode - human readable output
	if os.Getenv("HELVILETTE_DEV") == "1" {
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		logger = zerolog.New(output).With().Timestamp().Logger()
	} else {
		// Production mode - JSON output
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	// Set global log level from env
	level := os.Getenv("HELVILETTE_LOG_LEVEL")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// WithComponent returns a logger with component field
func WithComponent(component string) zerolog.Logger {
	return logger.With().Str("component", component).Logger()
}

// WithNodeID returns a logger with node_id field
func WithNodeID(nodeID string) zerolog.Logger {
	return logger.With().Str("node_id", nodeID).Logger()
}

// Debug logs a debug message
func Debug() *zerolog.Event {
	return logger.Debug()
}

// Info logs an info message
func Info() *zerolog.Event {
	return logger.Info()
}

// Warn logs a warning message
func Warn() *zerolog.Event {
	return logger.Warn()
}

// Error logs an error message
func Error() *zerolog.Event {
	return logger.Error()
}

// Fatal logs a fatal message and exits
func Fatal() *zerolog.Event {
	return logger.Fatal()
}

// Logger returns the underlying logger for advanced usage
func Logger() zerolog.Logger {
	return logger
}
