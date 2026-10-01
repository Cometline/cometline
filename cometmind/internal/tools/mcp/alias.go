package mcp

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

type (
	Workspace = toolkit.Workspace
	ToolSpec  = toolkit.ToolSpec
	Result    = toolkit.Result
	Tool      = toolkit.Tool
)

const RuntimePrefix = toolkit.RuntimePrefix

func WithToolSession(ctx context.Context, sessionID string) context.Context {
	return toolkit.WithToolSession(ctx, sessionID)
}
