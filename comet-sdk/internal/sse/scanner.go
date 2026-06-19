// Package sse provides a simple Server-Sent Events line scanner.
// It is shared by all provider stream parsers.
package sse

import (
	"bufio"
	"io"
	"strings"
)

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
	return &Scanner{scanner: bufio.NewScanner(r)}
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
