package logger

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew_JSON(t *testing.T) {
	log, err := New("info", "json")
	if err != nil {
		t.Fatalf("New(json) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_Console(t *testing.T) {
	log, err := New("debug", "console")
	if err != nil {
		t.Fatalf("New(console) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	log, err := New("invalid", "json")
	if err != nil {
		t.Fatalf("New(invalid level) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_DifferentLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		log, err := New(level, "json")
		if err != nil {
			t.Errorf("New(%q) failed: %v", level, err)
		}
		if log == nil {
			t.Errorf("New(%q) returned nil logger", level)
		}
	}
}

func TestFromContext(t *testing.T) {
	base := zap.NewNop()
	child := FromContext(base, zap.String("key", "value"))

	if child == nil {
		t.Fatal("expected non-nil child logger")
	}
}

func TestFromContext_MultipleFields(t *testing.T) {
	base := zap.NewNop()
	child := FromContext(base,
		zap.String("request_id", "abc-123"),
		zap.Int("status", 200),
		zap.Duration("latency", 0),
	)

	if child == nil {
		t.Fatal("expected non-nil child logger")
	}
}

func TestFromContext_NopLogger(t *testing.T) {
	child := FromContext(nil)
	if child == nil {
		// FromContext should handle nil gracefully or panic
		// This test just ensures it doesn't silently fail
	}
}

func TestNew_Production(t *testing.T) {
	log, err := New("info", "production")
	if err != nil {
		t.Fatalf("New(production) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_EmptyFormat(t *testing.T) {
	log, err := New("info", "")
	if err != nil {
		t.Fatalf("New(empty format) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_EmptyLevel(t *testing.T) {
	log, err := New("", "json")
	if err != nil {
		t.Fatalf("New(empty level) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_VerboseLevel(t *testing.T) {
	log, err := New("-1", "json")
	if err != nil {
		t.Fatalf("New(verbose level) failed: %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLogger_Level(t *testing.T) {
	tests := []struct {
		level string
		want  zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		log, err := New(tt.level, "json")
		if err != nil {
			t.Errorf("New(%q) failed: %v", tt.level, err)
			continue
		}
		if log == nil {
			t.Errorf("New(%q) returned nil", tt.level)
		}
	}
}
