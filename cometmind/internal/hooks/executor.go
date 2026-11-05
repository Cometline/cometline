package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/process"
	"go.uber.org/zap"
)

type commandRunner interface {
	Run(ctx context.Context, handler resolvedHandler, env ToolEnv, workspacePath string) (Outcome, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, handler resolvedHandler, env ToolEnv, workspacePath string) (Outcome, error) {
	timeout := handler.TimeoutSec
	if timeout <= 0 {
		timeout = defaultTimeoutSec
	}
	if timeout > maxTimeoutSec {
		timeout = maxTimeoutSec
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmdLine := resolveCommand(handler.Command, handler.WorkDir)
	cmd := process.CommandContext(runCtx, "sh", "-c", cmdLine)
	cmd.Dir = strings.TrimSpace(workspacePath)
	if cmd.Dir == "" {
		cmd.Dir = strings.TrimSpace(env.WorkspacePath)
	}

	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(env); err != nil {
		return Outcome{}, err
	}
	cmd.Stdin = &stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if stderr.Len() > 0 {
		logging.L().Warn("hooks.stderr",
			zap.String("event", env.HookEvent),
			zap.String("tool", env.ToolName),
			zap.String("source", handler.Source),
			zap.String("stderr", strings.TrimSpace(stderr.String())),
		)
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case 2:
				reason := strings.TrimSpace(stderr.String())
				if reason == "" {
					reason = "hook blocked (exit 2)"
				}
				return Outcome{Block: true, BlockReason: reason}, nil
			}
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			logging.L().Warn("hooks.timeout",
				zap.String("event", env.HookEvent),
				zap.String("source", handler.Source),
				zap.Int("timeout_sec", timeout),
			)
		} else {
			logging.L().Warn("hooks.failed",
				zap.String("event", env.HookEvent),
				zap.String("source", handler.Source),
				zap.Error(err),
			)
		}
		return Outcome{}, err
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return Outcome{}, nil
	}
	var parsed Outcome
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		logging.L().Warn("hooks.bad_stdout_json",
			zap.String("event", env.HookEvent),
			zap.String("source", handler.Source),
			zap.Error(err),
		)
		return Outcome{}, err
	}
	return parsed, nil
}

func mergeToolInput(base, patch json.RawMessage) json.RawMessage {
	if len(patch) == 0 || string(patch) == "null" {
		return base
	}
	var baseMap map[string]json.RawMessage
	if len(base) > 0 {
		_ = json.Unmarshal(base, &baseMap)
	}
	if baseMap == nil {
		baseMap = map[string]json.RawMessage{}
	}
	var patchMap map[string]json.RawMessage
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return patch
	}
	for k, v := range patchMap {
		baseMap[k] = v
	}
	out, err := json.Marshal(baseMap)
	if err != nil {
		return base
	}
	return out
}

func applyOutcome(acc *PreResult, o Outcome) {
	if o.Block {
		acc.Blocked = true
		if strings.TrimSpace(o.BlockReason) != "" {
			acc.BlockReason = strings.TrimSpace(o.BlockReason)
		} else if acc.BlockReason == "" {
			acc.BlockReason = "hook blocked tool"
		}
	}
	if len(o.ModifyToolInput) > 0 && string(o.ModifyToolInput) != "null" {
		acc.Input = mergeToolInput(acc.Input, o.ModifyToolInput)
	}
	if s := strings.TrimSpace(o.AppendSystem); s != "" {
		acc.AppendSystem = append(acc.AppendSystem, s)
	}
}

func runChain(ctx context.Context, runner commandRunner, handlers []resolvedHandler, env ToolEnv, workspacePath string, acc *PreResult) {
	depth := 0
	for _, h := range handlers {
		if depth >= maxChainDepth {
			logging.L().Warn("hooks.max_depth", zap.String("event", env.HookEvent))
			break
		}
		if !matcherMatches(h.Matcher, env.ToolName) {
			continue
		}
		depth++
		env.ToolInput = acc.Input
		out, err := runner.Run(ctx, h, env, workspacePath)
		if err != nil {
			continue
		}
		applyOutcome(acc, out)
		if acc.Blocked {
			return
		}
	}
}

func runPostChain(ctx context.Context, runner commandRunner, handlers []resolvedHandler, env ToolEnv, workspacePath string) []string {
	var appendSystem []string
	depth := 0
	for _, h := range handlers {
		if depth >= maxChainDepth {
			break
		}
		if !matcherMatches(h.Matcher, env.ToolName) {
			continue
		}
		depth++
		out, err := runner.Run(ctx, h, env, workspacePath)
		if err != nil {
			continue
		}
		if out.Block {
			continue
		}
		if s := strings.TrimSpace(out.AppendSystem); s != "" {
			appendSystem = append(appendSystem, s)
		}
	}
	return appendSystem
}

// noopRunner for tests.
type noopReader struct{}

func (noopReader) Run(_ context.Context, _ resolvedHandler, _ ToolEnv, _ string) (Outcome, error) {
	return Outcome{}, io.EOF
}
