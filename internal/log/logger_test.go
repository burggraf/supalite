package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		level:  LevelInfo,
		writer: &buf,
	}

	tests := []struct {
		name       string
		level      Level
		shouldLog  bool
		logFunc    func()
	}{
		{"Debug not logged at Info level", LevelDebug, false, func() {
			logger.log(LevelDebug, "debug message")
		}},
		{"Info logged at Info level", LevelInfo, true, func() {
			logger.log(LevelInfo, "info message")
		}},
		{"Warn logged at Info level", LevelWarn, true, func() {
			logger.log(LevelWarn, "warn message")
		}},
		{"Error logged at Info level", LevelError, true, func() {
			logger.log(LevelError, "error message")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("Expected hasOutput=%v, got %v", tt.shouldLog, hasOutput)
			}
		})
	}
}

func TestSetLevel(t *testing.T) {
	logger := &Logger{
		level:  LevelInfo,
		writer: &bytes.Buffer{},
	}

	if logger.level != LevelInfo {
		t.Errorf("Expected initial level Info, got %v", logger.level)
	}

	logger.SetLevel(LevelDebug)
	if logger.level != LevelDebug {
		t.Errorf("Expected level Debug after SetLevel, got %v", logger.level)
	}
}

func TestSetWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	logger := &Logger{
		level:  LevelInfo,
		writer: &buf1,
	}

	logger.log(LevelInfo, "test message 1")
	if !strings.Contains(buf1.String(), "test message 1") {
		t.Error("Expected message in buf1")
	}

	logger.SetWriter(&buf2)
	logger.log(LevelInfo, "test message 2")
	if !strings.Contains(buf2.String(), "test message 2") {
		t.Error("Expected message in buf2")
	}
}

func TestGlobalLogger(t *testing.T) {
	// Test that package functions work
	SetLevel(LevelDebug)
	SetLevel(LevelInfo)

	// These should not panic
	Debug("debug test")
	Info("info test")
	Warn("warn test")
	Error("error test")
}

func TestLogFormat(t *testing.T) {
	var buf bytes.Buffer

	logger := &Logger{
		level:  LevelInfo,
		writer: &buf,
	}

	logger.log(LevelInfo, "test message", "key", "value")
	output := buf.String()

	if !strings.Contains(output, "[INFO]") {
		t.Error("Expected [INFO] prefix in output")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Expected message in output")
	}
}
