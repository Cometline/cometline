package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/planning"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

func newPlanToolTestDeps(t *testing.T) (*planning.Service, string) {
	t.Helper()
	ctx := context.Background()
	conn, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	sessions := session.New(conn)
	ws, err := sessions.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	return planning.NewService(conn), sess.ID
}

func TestPlanToolsPersistCurrentSessionPlan(t *testing.T) {
	ctx := context.Background()
	plans, sessionID := newPlanToolTestDeps(t)
	r := NewRegistry(t.TempDir(), RegistryOptions{Planning: plans, SessionID: sessionID})
	if !r.Has("plan_write") || !r.Has("plan_update") {
		t.Fatalf("planning tools were not registered")
	}

	writeInput := json.RawMessage(`{"steps":[{"description":"inspect","status":"completed"},{"description":"patch","status":"in_progress"}]}`)
	res, err := r.Execute(ctx, "plan_write", writeInput)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "2 steps") {
		t.Fatalf("plan_write result=%+v", res)
	}

	updateInput := json.RawMessage(`{"step_index":1,"status":"blocked","blocker_reason":"needs decision"}`)
	res, err = r.Execute(ctx, "plan_update", updateInput)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("plan_update result=%+v", res)
	}
	steps, err := plans.GetPlan(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[1].Status != planning.StatusBlocked || steps[1].BlockerReason != "needs decision" {
		t.Fatalf("steps=%+v", steps)
	}
}

func TestPlanningToolsRequireSession(t *testing.T) {
	plans, _ := newPlanToolTestDeps(t)
	r := NewRegistry(t.TempDir(), RegistryOptions{Planning: plans})
	if r.Has("plan_write") || r.Has("plan_update") {
		t.Fatalf("planning tools registered without session id")
	}
}
