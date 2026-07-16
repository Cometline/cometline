package sse

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestScanner_AllowsLargeEventLines(t *testing.T) {
	large := strings.Repeat("x", 128*1024)
	s := NewScanner(strings.NewReader("event: response.created\ndata: " + large + "\n\n"))

	if !s.Next() {
		t.Fatalf("expected large event, err=%v", s.Err())
	}

	got := s.Event()
	if got.Type != "response.created" {
		t.Fatalf("type = %q, want response.created", got.Type)
	}
	if got.Data != large {
		t.Fatalf("data length = %d, want %d", len(got.Data), len(large))
	}
	if s.Next() {
		t.Fatalf("unexpected second event")
	}
	if err := s.Err(); err != nil {
		t.Fatalf("unexpected scanner err: %v", err)
	}
}

func TestIdleScannerReportsStalledStream(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()

	s := NewIdleScanner(reader, 10*time.Millisecond)
	defer s.Close()
	if s.Next() {
		t.Fatal("unexpected event")
	}

	var idle *cometsdk.StreamIdleTimeoutError
	if !errors.As(s.Err(), &idle) {
		t.Fatalf("Err() = %v, want StreamIdleTimeoutError", s.Err())
	}
	if idle.Duration != 10*time.Millisecond {
		t.Fatalf("duration = %s, want %s", idle.Duration, 10*time.Millisecond)
	}
}
