package logcat

import (
	"context"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"V", LevelVerbose},
		{"D", LevelDebug},
		{"I", LevelInfo},
		{"W", LevelWarn},
		{"WARN", LevelWarn},
		{"E", LevelError},
		{"F", LevelFatal},
		{"unknown", LevelVerbose},
	}

	for _, tt := range tests {
		actual := ParseLevel(tt.input)
		if actual != tt.expected {
			t.Errorf("input %s: got %v, want %v", tt.input, actual, tt.expected)
		}
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantTime    string
		wantLevel   Level
		wantTag     string
		wantPID     string
		wantMessage string
	}{
		{
			name:        "Standard Logcat Time Line",
			raw:         "10-31 00:15:32.123 D/ActivityManager( 1234): Starting activity",
			wantTime:    "10-31 00:15:32.123",
			wantLevel:   LevelDebug,
			wantTag:     "ActivityManager",
			wantPID:     "1234",
			wantMessage: "Starting activity",
		},
		{
			name:        "Error without PID",
			raw:         "10-31 00:15:32.123 E/AndroidRuntime: NullPointerException occurred",
			wantTime:    "10-31 00:15:32.123",
			wantLevel:   LevelError,
			wantTag:     "AndroidRuntime",
			wantPID:     "",
			wantMessage: "NullPointerException occurred",
		},
		{
			name:        "Threadtime Format Line",
			raw:         "10-31 00:15:32.123  1234  5678 W System.err: Warning message",
			wantTime:    "10-31 00:15:32.123",
			wantLevel:   LevelWarn,
			wantTag:     "System.err",
			wantPID:     "1234",
			wantMessage: "Warning message",
		},
		{
			name:        "Generic Text Line",
			raw:         "--------- beginning of main",
			wantTime:    "",
			wantLevel:   LevelVerbose,
			wantTag:     "",
			wantPID:     "",
			wantMessage: "--------- beginning of main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ParseLine(tt.raw)
			if entry.Time != tt.wantTime {
				t.Errorf("Time: got %q, want %q", entry.Time, tt.wantTime)
			}
			if entry.Level != tt.wantLevel {
				t.Errorf("Level: got %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Tag != tt.wantTag {
				t.Errorf("Tag: got %q, want %q", entry.Tag, tt.wantTag)
			}
			if entry.PID != tt.wantPID {
				t.Errorf("PID: got %q, want %q", entry.PID, tt.wantPID)
			}
			if entry.Message != tt.wantMessage {
				t.Errorf("Message: got %q, want %q", entry.Message, tt.wantMessage)
			}
		})
	}
}

func TestColorize_RespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	entry := ParseLine("10-31 00:15:32.123 E/AndroidRuntime( 1234): Crash")
	out := Colorize(entry)
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escape sequences when NO_COLOR is set, got %q", out)
	}
}

func TestStream_ReportsSubprocessError(t *testing.T) {
	// Invoke `go` with an invalid subcommand so the subprocess starts and exits non-zero with 0 stdout lines.
	err := Stream(context.Background(), StreamOptions{
		ADBBinary: "go",
		Serial:    "nonexistent",
	}, nil)
	if err == nil {
		t.Errorf("expected Stream to return an error when subprocess exits non-zero, got nil")
	}

	_, crashErr := DumpCrashLogs(context.Background(), "go", "nonexistent", "", nil)
	if crashErr == nil {
		t.Errorf("expected DumpCrashLogs to return an error when subprocess exits non-zero, got nil")
	}
}
