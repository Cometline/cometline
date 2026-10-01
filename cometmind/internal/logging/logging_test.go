package logging

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		raw   string
		level zapcore.Level
		err   bool
	}{
		{"debug", zapcore.DebugLevel, false},
		{"INFO", zapcore.InfoLevel, false},
		{"warn", zapcore.WarnLevel, false},
		{"warning", zapcore.WarnLevel, false},
		{"error", zapcore.ErrorLevel, false},
		{"", zapcore.ErrorLevel, false},
		{"verbose", zapcore.ErrorLevel, true},
	}

	for _, tt := range tests {
		level, err := ParseLevel(tt.raw)
		if tt.err {
			if err == nil {
				t.Fatalf("ParseLevel(%q) expected error", tt.raw)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLevel(%q) error = %v", tt.raw, err)
		}
		if level != tt.level {
			t.Fatalf("ParseLevel(%q) = %v, want %v", tt.raw, level, tt.level)
		}
	}
}

func TestInitFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(&buf),
		zapcore.ErrorLevel,
	)
	logger = zap.New(core)

	logger.Info("should-not-appear")
	logger.Error("should-appear")

	out := buf.String()
	if strings.Contains(out, "should-not-appear") {
		t.Fatalf("info log leaked at error level: %q", out)
	}
	if !strings.Contains(out, "should-appear") {
		t.Fatalf("error log missing at error level: %q", out)
	}
}
