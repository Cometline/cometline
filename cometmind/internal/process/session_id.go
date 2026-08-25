package process

import "context"

type sessionIDKey struct{}

// WithSessionID attaches a chat session id used to load a terminal env snapshot.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFrom returns the session id attached by WithSessionID.
func SessionIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey{}).(string)
	return v
}
