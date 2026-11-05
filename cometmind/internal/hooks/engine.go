package hooks

import (
	"context"
	"encoding/json"

	"github.com/Cometline/cometline/cometmind/internal/config"
)

// Engine runs PreToolUse and PostToolUse subprocess hooks for one workspace.
type Engine struct {
	enabled   bool
	workspace string
	projectOK bool
	handlers  []resolvedHandler
	runner    commandRunner
}

// NewEngine loads hook definitions for workspacePath. Hooks stay inactive until Enabled is true.
func NewEngine(cfg config.HooksConfig, workspacePath string) *Engine {
	projectOK := WorkspaceTrusted(workspacePath, cfg.TrustedWorkspacePaths)
	return &Engine{
		enabled:   cfg.Enabled,
		workspace: workspacePath,
		projectOK: projectOK,
		handlers:  discoverHandlers(workspacePath, projectOK),
		runner:    execRunner{},
	}
}

// WithRunner replaces subprocess execution (tests).
func (e *Engine) WithRunner(r commandRunner) *Engine {
	if e != nil {
		e.runner = r
	}
	return e
}

// Enabled reports whether hooks are globally enabled in settings.
func (e *Engine) Enabled() bool {
	return e != nil && e.enabled
}

// Catalog returns discovered hook handlers (for settings / API).
func (e *Engine) Catalog() []CatalogEntry {
	if e == nil {
		return nil
	}
	out := make([]CatalogEntry, 0, len(e.handlers))
	for _, h := range e.handlers {
		if h.Event != EventPreToolUse && h.Event != EventPostToolUse {
			continue
		}
		out = append(out, CatalogEntry{
			Event:   h.Event,
			Matcher: h.Matcher,
			Command: h.Command,
			Source:  h.Source,
		})
	}
	return out
}

// RunPreToolUse executes matching PreToolUse hooks. Failures are fail-open.
func (e *Engine) RunPreToolUse(ctx context.Context, env ToolEnv) PreResult {
	res := PreResult{Input: env.ToolInput}
	if e == nil || !e.enabled || len(e.handlers) == 0 {
		return res
	}
	env.HookEvent = EventPreToolUse
	if res.Input == nil {
		res.Input = json.RawMessage(`{}`)
	}
	var pre []resolvedHandler
	for _, h := range e.handlers {
		if h.Event == EventPreToolUse {
			pre = append(pre, h)
		}
	}
	runChain(ctx, e.runner, pre, env, e.workspace, &res)
	return res
}

// WorkspacePath returns the workspace root for hook subprocess cwd.
func (e *Engine) WorkspacePath() string {
	if e == nil {
		return ""
	}
	return e.workspace
}

// RunPostToolUse executes matching PostToolUse hooks after tool execution.
func (e *Engine) RunPostToolUse(ctx context.Context, env ToolEnv) PostResult {
	if e == nil || !e.enabled || len(e.handlers) == 0 {
		return PostResult{}
	}
	env.HookEvent = EventPostToolUse
	var post []resolvedHandler
	for _, h := range e.handlers {
		if h.Event == EventPostToolUse {
			post = append(post, h)
		}
	}
	appendSystem := runPostChain(ctx, e.runner, post, env, e.workspace)
	return PostResult{AppendSystem: appendSystem}
}
