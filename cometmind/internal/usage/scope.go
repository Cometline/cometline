package usage

import (
	"context"
	"strings"
)

type scopeKey struct{}

// Scope attributes a usage event to a workspace and session.
type Scope struct {
	WorkspaceID string
	SessionID   string
}

// WithScope stores workspace/session attribution for nested LLM and embedding calls.
func WithScope(ctx context.Context, workspaceID, sessionID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if workspaceID == "" && sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, Scope{WorkspaceID: workspaceID, SessionID: sessionID})
}

// ScopeFrom returns the usage attribution stored on ctx, if any.
func ScopeFrom(ctx context.Context) Scope {
	if ctx == nil {
		return Scope{}
	}
	scope, _ := ctx.Value(scopeKey{}).(Scope)
	return scope
}
