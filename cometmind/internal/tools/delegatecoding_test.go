package tools

import (
	"context"
	"io"
	"testing"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

func TestDelegateCodingTaskWithFakeACP(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := store.OpenSQLite(ctx, t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	svc := session.New(db)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	parent, err := svc.NewSession(ctx, ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	mgr := acp.NewSessionManager(acp.DefaultConfig())
	mgr.ProcessStarter = func(ctx context.Context, cfg acp.Config) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
		return acp.StartFakeAgentPipes(ctx)
	}
	tool := DelegateCodingTask{
		Workspace: Workspace{Root: ws.Path},
		Sessions:  svc,
		ACP:       acp.DefaultConfig(),
		ACPMgr:    mgr,
	}

	toolCtx := WithToolSession(ctx, parent.ID)
	res, err := tool.Execute(toolCtx, []byte(`{"task":"add a comment","verify_command":"true"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not ok: %s", res.Output)
	}

	children, err := svc.ListChildSessions(ctx, parent.ID)
	if err != nil {
		t.Fatalf("children: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("children = %d, want 1", len(children))
	}
	if children[0].DelegationStatus != "completed" {
		t.Fatalf("status = %q", children[0].DelegationStatus)
	}
}
