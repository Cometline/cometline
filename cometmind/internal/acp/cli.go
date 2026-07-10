package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/cometline/cometmind/internal/process"
)

// CLIProcessStarter starts a non-interactive coding harness and returns its
// stdout, stderr, and process closer. It is injectable so parser and
// cancellation behavior can be tested without installing a coding CLI.
type CLIProcessStarter func(
	ctx context.Context,
	cfg Config,
	workspaceRoot string,
	prompt string,
) (stdout io.ReadCloser, stderr io.ReadCloser, closer io.Closer, err error)

func defaultCLIProcessStarter(
	ctx context.Context,
	cfg Config,
	workspaceRoot string,
	prompt string,
) (io.ReadCloser, io.ReadCloser, io.Closer, error) {
	cfg = cfg.normalized()
	commandName, args := cfg.commandArgs()
	command, err := resolveAgentCommand(commandName)
	if err != nil {
		return nil, nil, nil, err
	}

	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workspaceRoot
	cmd.Env = process.Env()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	return stdout, stderr, &cmdWaitCloser{cmd: cmd}, nil
}

func resolveAgentCommand(command string) (string, error) {
	resolved, err := process.ResolveCommand(command)
	if err != nil {
		return "", fmt.Errorf("resolve coding harness command %q: %w", command, process.CommandNotFoundError(command, err))
	}
	return resolved, nil
}

type cliEvent struct {
	Update ProgressUpdate
	Final  string
}

func parseCLIEvent(harness Harness, line string) cliEvent {
	line = strings.TrimSpace(line)
	if line == "" {
		return cliEvent{}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return cliEvent{Update: ProgressUpdate{Kind: "message", Content: line}}
	}

	typ, _ := payload["type"].(string)
	switch ParseHarness(string(harness)) {
	case HarnessOpenCode:
		return parseOpenCodeEvent(typ, payload)
	case HarnessClaude:
		return parseClaudeEvent(typ, payload)
	case HarnessCodex:
		return parseCodexEvent(typ, payload)
	default:
		return parseGenericCLIEvent(payload)
	}
}

func parseOpenCodeEvent(typ string, payload map[string]any) cliEvent {
	part, _ := payload["part"].(map[string]any)
	switch typ {
	case "text":
		return messageEvent(textFromValue(part))
	case "reasoning":
		return cliEvent{Update: ProgressUpdate{Kind: "thought", Content: strings.TrimSpace(textFromValue(part))}}
	case "tool_use":
		title := firstString(part, "tool", "name", "command", "path")
		state, _ := part["state"].(map[string]any)
		status := statusFromPayload(state, "running")
		return cliEvent{Update: ProgressUpdate{Kind: "tool_call", Title: title, Status: status}}
	case "step_start":
		return cliEvent{Update: ProgressUpdate{Kind: "status", Title: "step started", Status: "running"}}
	case "step_finish":
		return cliEvent{Update: ProgressUpdate{Kind: "status", Title: "step finished", Status: "completed"}}
	case "error":
		return cliEvent{Update: ProgressUpdate{Kind: "status", Title: textFromValue(payload["error"]), Status: "failed"}}
	default:
		return parseGenericCLIEvent(payload)
	}
}

func parseClaudeEvent(typ string, payload map[string]any) cliEvent {
	switch typ {
	case "assistant":
		text := textFromValue(payload["message"])
		return messageEvent(text)
	case "content_block_delta":
		text := textFromValue(payload["delta"])
		return messageEvent(text)
	case "tool_use":
		name, _ := payload["name"].(string)
		return cliEvent{Update: ProgressUpdate{Kind: "tool_call", Title: name, Status: "running"}}
	case "result":
		result := textFromValue(payload["result"])
		return cliEvent{Final: result}
	default:
		return parseGenericCLIEvent(payload)
	}
}

func parseCodexEvent(typ string, payload map[string]any) cliEvent {
	item, _ := payload["item"].(map[string]any)
	if item != nil {
		itemType, _ := item["type"].(string)
		switch itemType {
		case "agent_message", "assistant_message":
			return messageEvent(textFromValue(item))
		case "command_execution", "shell_command", "mcp_tool_call", "file_change":
			title := firstString(item, "command", "name", "path", "tool")
			return cliEvent{Update: ProgressUpdate{Kind: "tool_call", Title: title, Status: statusFromPayload(item, "running")}}
		}
	}
	if typ == "turn.completed" {
		return cliEvent{Update: ProgressUpdate{Kind: "status", Title: "turn completed", Status: "completed"}}
	}
	return parseGenericCLIEvent(payload)
}

func parseGenericCLIEvent(payload map[string]any) cliEvent {
	if text := firstString(payload, "text", "result", "message", "output"); text != "" {
		return messageEvent(text)
	}
	if typ, _ := payload["type"].(string); typ != "" {
		return cliEvent{Update: ProgressUpdate{Kind: "status", Title: typ}}
	}
	return cliEvent{}
}

func messageEvent(text string) cliEvent {
	return cliEvent{Update: ProgressUpdate{Kind: "message", Content: strings.TrimSpace(text)}}
}

func statusFromPayload(payload map[string]any, fallback string) string {
	if status, ok := payload["status"].(string); ok && strings.TrimSpace(status) != "" {
		return status
	}
	return fallback
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if text := strings.TrimSpace(textFromValue(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func textFromValue(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case []any:
		var parts []string
		for _, item := range value {
			if text := strings.TrimSpace(textFromValue(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "result", "content", "message", "output"} {
			if nested, ok := value[key]; ok {
				if text := strings.TrimSpace(textFromValue(nested)); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func collectCLIOutput(
	ctx context.Context,
	harness Harness,
	stdout io.Reader,
	stderr io.Reader,
	closer io.Closer,
	onProgress func(ProgressUpdate),
) (summary string, stderrText string, scanErr error, waitErr error) {
	var chunks []string
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stderrBuf, stderr)
		close(stderrDone)
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var final string
	for scanner.Scan() {
		event := parseCLIEvent(harness, scanner.Text())
		if event.Update.Content != "" {
			chunks = append(chunks, event.Update.Content)
		}
		if event.Final != "" {
			final = event.Final
		}
		if onProgress != nil && (event.Update.Content != "" || event.Update.Title != "") {
			onProgress(event.Update)
		}
	}
	scanErr = scanner.Err()
	waitErr = closer.Close()
	select {
	case <-stderrDone:
	case <-ctx.Done():
		<-stderrDone
	}

	if final != "" {
		summary = final
	} else {
		summary = strings.TrimSpace(strings.Join(chunks, "\n"))
	}
	return summary, strings.TrimSpace(stderrBuf.String()), scanErr, waitErr
}
