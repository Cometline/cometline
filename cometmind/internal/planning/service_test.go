package planning

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

func newPlanningTestService(t *testing.T) (*Service, session.Session) {
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
	return NewService(conn), sess
}

func TestSetGetUpdatePlan(t *testing.T) {
	ctx := context.Background()
	svc, sess := newPlanningTestService(t)

	steps, err := svc.SetPlan(ctx, sess.ID, []StepInput{
		{Description: "inspect code", Status: StatusCompleted},
		{Description: "write fix", Status: StatusInProgress},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0].StepIndex != 0 || steps[1].StepIndex != 1 {
		t.Fatalf("steps=%+v", steps)
	}

	updated, err := svc.UpdateStep(ctx, sess.ID, 1, StatusBlocked, "waiting on credentials")
	if err != nil {
		t.Fatal(err)
	}
	if updated[1].Status != StatusBlocked || updated[1].BlockerReason != "waiting on credentials" {
		t.Fatalf("updated=%+v", updated)
	}

	block := FormatPromptBlock(updated)
	if !strings.Contains(block, "## Current Plan") || !strings.Contains(block, "waiting on credentials") {
		t.Fatalf("prompt block=%q", block)
	}
}

func TestSetPlanReplacesExistingPlan(t *testing.T) {
	ctx := context.Background()
	svc, sess := newPlanningTestService(t)
	if _, err := svc.SetPlan(ctx, sess.ID, []StepInput{{Description: "first"}, {Description: "second"}}); err != nil {
		t.Fatal(err)
	}
	replaced, err := svc.SetPlan(ctx, sess.ID, []StepInput{{Description: "replacement"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(replaced) != 1 || replaced[0].Description != "replacement" || replaced[0].StepIndex != 0 {
		t.Fatalf("replaced=%+v", replaced)
	}
}

func TestPlanValidation(t *testing.T) {
	ctx := context.Background()
	svc, sess := newPlanningTestService(t)
	if _, err := svc.SetPlan(ctx, sess.ID, []StepInput{{Description: "bad", Status: "nope"}}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err=%v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetPlan(ctx, sess.ID, []StepInput{{Description: "   "}}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err=%v, want ErrInvalidInput", err)
	}
}
