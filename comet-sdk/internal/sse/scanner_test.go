package sse_test

import (
	"strings"
	"testing"

	"github.com/cometline/comet-sdk/internal/sse"
	"github.com/stretchr/testify/require"
)

func TestScanner_SingleDataEvent(t *testing.T) {
	input := "data: hello\n\n"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "", ev.Type)
	require.Equal(t, "hello", ev.Data)

	require.False(t, s.Next())
	require.NoError(t, s.Err())
}

func TestScanner_EventTypeAndData(t *testing.T) {
	input := "event: content_block_delta\ndata: {\"type\":\"text_delta\"}\n\n"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "content_block_delta", ev.Type)
	require.Equal(t, `{"type":"text_delta"}`, ev.Data)

	require.False(t, s.Next())
}

func TestScanner_MultipleEvents(t *testing.T) {
	input := strings.Join([]string{
		"event: ping",
		"data: first",
		"",
		"event: pong",
		"data: second",
		"",
	}, "\n")

	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "ping", ev.Type)
	require.Equal(t, "first", ev.Data)

	require.True(t, s.Next())
	ev = s.Event()
	require.Equal(t, "pong", ev.Type)
	require.Equal(t, "second", ev.Data)

	require.False(t, s.Next())
}

func TestScanner_MultiLineData(t *testing.T) {
	// Multiple data: lines in one event block are joined with "\n".
	input := "data: line1\ndata: line2\ndata: line3\n\n"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "line1\nline2\nline3", ev.Data)
}

func TestScanner_CommentsIgnored(t *testing.T) {
	input := ": this is a comment\ndata: actual\n\n"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "actual", ev.Data)
}

func TestScanner_EmptyBlocksSkipped(t *testing.T) {
	// Two blank lines in a row should not emit an empty event.
	input := "\n\ndata: real\n\n"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "real", ev.Data)

	require.False(t, s.Next())
}

func TestScanner_NoTrailingBlankLine(t *testing.T) {
	// Stream ended without trailing blank line — last event should still be emitted.
	input := "data: last"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	ev := s.Event()
	require.Equal(t, "last", ev.Data)

	require.False(t, s.Next())
}

func TestScanner_LeadingSpaceStripped(t *testing.T) {
	// SSE spec: a single leading space after the colon is stripped.
	input := "data: with space\n\n"
	s := sse.NewScanner(strings.NewReader(input))

	require.True(t, s.Next())
	require.Equal(t, "with space", s.Event().Data)
}

func TestScanner_EmptyReader(t *testing.T) {
	s := sse.NewScanner(strings.NewReader(""))
	require.False(t, s.Next())
	require.NoError(t, s.Err())
}
