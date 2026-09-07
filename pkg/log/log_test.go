package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestWithComponent(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	oldLogger := logger
	logger = zerolog.New(&buf)
	defer func() { logger = oldLogger }()

	componentLogger := WithComponent("test-component")
	componentLogger.Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"component":"test-component"`) {
		t.Errorf("expected component field in log output, got: %s", output)
	}
	if !strings.Contains(output, `"message":"test message"`) {
		t.Errorf("expected message in log output, got: %s", output)
	}
}

func TestWithNodeID(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	logger = zerolog.New(&buf)
	defer func() { logger = oldLogger }()

	nodeLogger := WithNodeID("node-123")
	nodeLogger.Info().Msg("test")

	output := buf.String()
	if !strings.Contains(output, `"node_id":"node-123"`) {
		t.Errorf("expected node_id field in log output, got: %s", output)
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	logger = zerolog.New(&buf)
	defer func() { logger = oldLogger }()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Debug should not appear at Info level
	Debug().Msg("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Error("debug message should not appear at Info level")
	}

	buf.Reset()
	Info().Msg("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Error("info message should appear at Info level")
	}

	buf.Reset()
	Warn().Msg("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("warn message should appear at Info level")
	}

	buf.Reset()
	Error().Msg("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("error message should appear at Info level")
	}
}

func TestLoggerJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	logger = zerolog.New(&buf).With().Timestamp().Logger()
	defer func() { logger = oldLogger }()

	Info().Str("key", "value").Msg("json test")

	// Verify output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Errorf("log output is not valid JSON: %v, output: %s", err, buf.String())
	}

	if result["message"] != "json test" {
		t.Errorf("expected message 'json test', got: %v", result["message"])
	}
	if result["key"] != "value" {
		t.Errorf("expected key 'value', got: %v", result["key"])
	}
}

func TestSetLevel(t *testing.T) {
	tests := []struct {
		input         string
		expectedLevel zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"warn", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"info", zerolog.InfoLevel},
		{"", zerolog.InfoLevel},        // default
		{"garbage", zerolog.InfoLevel}, // unrecognized defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			SetLevel(tt.input)
			if zerolog.GlobalLevel() != tt.expectedLevel {
				t.Errorf("SetLevel(%q): expected %v, got %v", tt.input, tt.expectedLevel, zerolog.GlobalLevel())
			}
		})
	}
}
