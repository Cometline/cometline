package tools

import (
	"context"

	"github.com/cometline/cometmind/internal/event"
)

type execCtxKey int

const (
	execKeySession execCtxKey = iota
	execKeyProgress
	execKeySessions
)

// ProgressFn emits runtime events during long-running tool execution.
type ProgressFn func(event.Event)

// WithToolSession attaches the active CometMind session id to the tool context.
func WithToolSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, execKeySession, sessionID)
}

// ToolSessionFrom returns the active session id when present.
func ToolSessionFrom(ctx context.Context) string {
	v, _ := ctx.Value(execKeySession).(string)
	return v
}

// WithProgress attaches a callback for streaming tool progress to the parent turn.
func WithProgress(ctx context.Context, fn ProgressFn) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, execKeyProgress, fn)
}

// ProgressFrom returns the progress callback when present.
func ProgressFrom(ctx context.Context) ProgressFn {
	fn, _ := ctx.Value(execKeyProgress).(ProgressFn)
	return fn
}
