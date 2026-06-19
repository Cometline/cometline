package acp_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/acp"
)

func TestAgentRunnerWithFakeAgent(t *testing.T) {
	t.Parallel()

	runner := acp.AgentRunner{
		ProcessStarter: func(ctx context.Context, cfg acp.Config) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
			return acp.StartFakeAgentPipes(ctx)
		},
	}

	result, err := runner.Run(context.Background(), acp.TaskRequest{
		WorkspaceRoot: t.TempDir(),
		Task:          "add logging",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q", result.Status)
	}
	if !strings.Contains(result.Summary, "completed:") {
		t.Fatalf("summary = %q", result.Summary)
	}
	if result.AgentName != "fake-opencode" {
		t.Fatalf("agent = %q", result.AgentName)
	}
}
