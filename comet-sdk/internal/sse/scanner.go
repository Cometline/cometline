// Package sse provides a simple Server-Sent Events line scanner.
// It is shared by all provider stream parsers.
package sse

import (
	"bufio"
	"io"
	"strings"
	"sync"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
)

const maxEventBytes = 16 * 1024 * 1024

// Event holds the parsed fields of a single SSE event.
// For simple providers that only use "data:" lines, only Data is populated.
type Event struct {
	// Type is the value of the "event:" field, e.g. "content_block_delta".
	// Empty string if no "event:" field was present.
	Type string

	// Data is the value of the "data:" field.
	// If multiple "data:" lines appear in one event block, they are joined with "\n".
	Data string
}

// Scanner reads SSE events from an io.Reader.
// Usage:
//
//	s := sse.NewScanner(body)
//	for s.Next() {
//	    ev := s.Event()
//	    // handle ev.Type, ev.Data
//	}
//	if err := s.Err(); err != nil { ... }
type Scanner struct {
	scanner *bufio.Scanner
	current Event
	err     error
	done    bool
}

// NewScanner creates a new Scanner reading from r.
func NewScanner(r io.Reader) *Scanner {
	scanner := bufio.NewScanner(r)
	// Some providers echo the complete request tool schema inside early stream
	// events. The default bufio.Scanner token limit is 64 KiB, which is too
	// small for a real agent registry with many MCP/function tools.
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventBytes)
	return &Scanner{scanner: scanner}
}

// Next advances the scanner to the next SSE event.
// It returns true if an event is available; false on EOF or error.
func (s *Scanner) Next() bool {
	if s.done {
		return false
	}

	var ev Event
	var dataLines []string

	for s.scanner.Scan() {
		line := s.scanner.Text()

		// Blank line = end of event block.
		if line == "" {
			// Only emit if we have at least a data field.
			if len(dataLines) > 0 || ev.Type != "" {
				ev.Data = strings.Join(dataLines, "\n")
				s.current = ev
				return true
			}
			// Reset and keep scanning (ignore empty blocks).
			ev = Event{}
			dataLines = dataLines[:0]
			continue
		}

		// Skip comment lines.
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, _ := strings.Cut(line, ":")
		// Trim a single leading space from value per SSE spec.
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			ev.Type = value
		case "data":
			dataLines = append(dataLines, value)
			// Ignore "id" and "retry" fields — not needed for LLM streaming.
		}
	}

	s.done = true
	if err := s.scanner.Err(); err != nil {
		s.err = err
		return false
	}

	// Emit a final event if the stream ended without a trailing blank line.
	if len(dataLines) > 0 || ev.Type != "" {
		ev.Data = strings.Join(dataLines, "\n")
		s.current = ev
		return true
	}

	return false
}

// Event returns the most recently parsed SSE event.
// Only valid after a call to Next() that returned true.
func (s *Scanner) Event() Event {
	return s.current
}

// Err returns the first non-EOF error encountered by the scanner.
func (s *Scanner) Err() error {
	return s.err
}

// IdleScanner closes a stream body that stops producing SSE events. It wraps a
// Scanner so existing parsing behavior remains unchanged for active streams.
type IdleScanner struct {
	*Scanner

	body    io.ReadCloser
	timeout time.Duration
	timer   *time.Timer

	mu       sync.Mutex
	closed   bool
	timedOut bool
}

// NewIdleScanner creates an SSE scanner with optional inactivity detection.
// A timeout of zero disables idle detection.
func NewIdleScanner(body io.ReadCloser, timeout time.Duration) *IdleScanner {
	s := &IdleScanner{
		Scanner: NewScanner(body),
		body:    body,
		timeout: timeout,
	}
	if timeout > 0 {
		s.timer = time.AfterFunc(timeout, s.expire)
	}
	return s
}

// Next advances to the next event and resets the inactivity timer on success.
func (s *IdleScanner) Next() bool {
	if !s.Scanner.Next() {
		return false
	}
	s.reset()
	return true
}

// Err reports an idle timeout instead of the body-close error used to unblock
// the underlying scanner.
func (s *IdleScanner) Err() error {
	s.mu.Lock()
	timedOut := s.timedOut
	s.mu.Unlock()
	if timedOut {
		return &cometsdk.StreamIdleTimeoutError{Duration: s.timeout}
	}
	return s.Scanner.Err()
}

// Close stops idle detection once the parser has reached a terminal event.
func (s *IdleScanner) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
	}
}

func (s *IdleScanner) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.timedOut || s.timer == nil {
		return
	}
	s.timer.Stop()
	s.timer.Reset(s.timeout)
}

func (s *IdleScanner) expire() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.timedOut = true
	s.mu.Unlock()
	_ = s.body.Close()
}
