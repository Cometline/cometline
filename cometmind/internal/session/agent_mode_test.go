package session

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func openTestService(t *testing.T) *Service {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	return New(conn)
}

func TestNewSessionDefaultsToAuto(t *testing.T) {
	t.Parallel()

	svc := openTestService(t)
	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if sess.AgentMode != string(AgentModeAuto) {
		t.Fatalf("agent mode = %q, want %q", sess.AgentMode, AgentModeAuto)
	}
}

func TestUpdateSessionAgentModePersists(t *testing.T) {
	t.Parallel()

	svc := openTestService(t)
	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	updated, err := svc.UpdateSessionAgentMode(context.Background(), sess.ID, AgentModePlan)
	if err != nil {
		t.Fatalf("UpdateSessionAgentMode() error = %v", err)
	}
	if updated.AgentMode != string(AgentModePlan) {
		t.Fatalf("updated agent mode = %q, want %q", updated.AgentMode, AgentModePlan)
	}

	reloaded, err := svc.GetSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if reloaded.AgentMode != string(AgentModePlan) {
		t.Fatalf("reloaded agent mode = %q, want %q", reloaded.AgentMode, AgentModePlan)
	}
}

func TestNewChildSessionInheritsParentAgentMode(t *testing.T) {
	t.Parallel()

	svc := openTestService(t)
	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	parent, err := svc.NewSession(context.Background(), ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	parent, err = svc.UpdateSessionAgentMode(context.Background(), parent.ID, AgentModePlan)
	if err != nil {
		t.Fatalf("UpdateSessionAgentMode() error = %v", err)
	}

	child, err := svc.NewChildSession(context.Background(), parent, "research", "general")
	if err != nil {
		t.Fatalf("NewChildSession() error = %v", err)
	}
	if child.AgentMode != string(AgentModePlan) {
		t.Fatalf("child agent mode = %q, want %q (inherited from plan parent)", child.AgentMode, AgentModePlan)
	}
}

func TestForkSessionStartsInAuto(t *testing.T) {
	t.Parallel()

	svc := openTestService(t)
	srcWS, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	src, err := svc.NewSession(context.Background(), srcWS.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	src, err = svc.UpdateSessionAgentMode(context.Background(), src.ID, AgentModePlan)
	if err != nil {
		t.Fatalf("UpdateSessionAgentMode() error = %v", err)
	}

	forked, err := svc.ForkSession(context.Background(), src.ID, t.TempDir())
	if err != nil {
		t.Fatalf("ForkSession() error = %v", err)
	}
	if forked.AgentMode != string(AgentModeAuto) {
		t.Fatalf("forked agent mode = %q, want %q (user forks always start in auto)", forked.AgentMode, AgentModeAuto)
	}
}
