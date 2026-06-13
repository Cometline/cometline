package session

import (
	"strings"
	"testing"
)

func TestTrimTranscriptToolOutput(t *testing.T) {
	t.Parallel()
	short := "ok"
	if trimTranscriptToolOutput(short) != short {
		t.Fatalf("short string trimmed unexpectedly")
	}
	long := strings.Repeat("x", 500)
	got := trimTranscriptToolOutput(long)
	want := strings.Repeat("x", 400) + "…"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
