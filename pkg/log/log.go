package log

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

// sink is the one writer every logger in the process ultimately reaches.
// Loggers derived by WithComponent capture the writer at derivation time, so
// the indirection is what lets SetOutput redirect them afterwards — without it
// a test could only capture loggers it built itself.
//
// Everything above the sink emits JSON; the sink's destination decides what
// becomes of it. That ordering matters: console formatting used to wrap the sink
// rather than sit behind it, which left no single point to swap.
var sink = &switchableWriter{out: os.Stdout}

type switchableWriter struct {
	mu  sync.RWMutex
	out io.Writer
}

func (w *switchableWriter) Write(p []byte) (int, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.out.Write(p)
}

func (w *switchableWriter) swap(out io.Writer) io.Writer {
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.out
	w.out = out
	return previous
}

// SetOutput redirects all log output and returns the previous destination, so
// callers can restore it. Intended for tests that assert on emitted fields.
//
//	var buf bytes.Buffer
//	defer log.SetOutput(log.SetOutput(&buf))
func SetOutput(out io.Writer) io.Writer {
	return sink.swap(out)
}

func init() {
	sink.out = defaultDestination()
	logger = zerolog.New(sink).With().Timestamp().Logger()

	// Set global log level from env
	SetLevel(os.Getenv("HELVILETTE_LOG_LEVEL"))
}

// defaultDestination decides where logs go when nothing has called SetOutput.
//
// journald first: run as a systemd unit there is no human at a terminal, and
// native fields are what make the entries queryable and collectable without
// configuring anything. Then HELVILETTE_DEV for a human who asked for prose.
// Then plain JSON on stdout, which is correct both for a terminal and for a
// plain container runtime that captures stdout. See ADR-0007 D2.
func defaultDestination() io.Writer {
	if underJournal() {
		return newJournalSink(IdentifierFor(os.Args[0]), os.Stderr)
	}

	if os.Getenv("HELVILETTE_DEV") == "1" {
		return zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}

	return os.Stdout
}

// SetLevel sets the global zerolog level from a string.
// Accepted values: "debug", "info", "warn", "error". Unrecognized values
// default to info. Call this from CLI flag parsing to connect --log-level
// to the logging subsystem.
//
// Usage:
//
//	log.SetLevel("debug")
func SetLevel(level string) {
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
