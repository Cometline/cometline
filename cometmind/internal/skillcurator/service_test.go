package skillcurator

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	_ "modernc.org/sqlite"
)

func TestNoteUseRestoresArchivedSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	name := "ship-checklist"
	root := filepath.Join(home, ".cometmind", "skills", name)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: ship-checklist\ndescription: Ship a reviewed change\nmetadata:\n  cometline:\n    origin: self-improvement\n---\n\n# Workflow\n"
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	conn := curatorTestDB(t)
	svc := New(conn)
	if err := svc.NoteUse(ctx, name); err != nil {
		t.Fatal(err)
	}
	if err := skills.ArchiveManagedSkill(name); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkArchived(ctx, name); err != nil {
		t.Fatal(err)
	}
	if err := svc.NoteUse(ctx, name); err != nil {
		t.Fatal(err)
	}
	status, pinned := svc.Lookup(ctx, name)
	if status != skills.CuratorStatusActive || pinned {
		t.Fatalf("status = %s pinned = %v", status, pinned)
	}
	row, err := svc.q.GetSkillCuratorState(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if row.ArchivedAt != 0 {
		t.Fatalf("archived_at = %d", row.ArchivedAt)
	}
	if row.LastUsedAt == 0 || row.UnusedSince == 0 {
		t.Fatalf("clock last_used=%d unused_since=%d", row.LastUsedAt, row.UnusedSince)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".cometmind", "skills", ".archive", name)); !os.IsNotExist(err) {
		t.Fatalf("archive entry = %v", err)
	}
}

func TestSkillCuratorKindFitsSessionCheck(t *testing.T) {
	ctx := context.Background()
	conn := curatorTestDB(t)
	if _, err := conn.ExecContext(ctx, `INSERT INTO workspaces (id, name, path) VALUES ('ws', 'ws', '/tmp/ws')`); err != nil {
		t.Fatal(err)
	}
	_, err := conn.ExecContext(ctx, `
		INSERT INTO sessions (id, workspace_id, model_id, provider_id, subagent_kind)
		VALUES ('child', 'ws', 'model', 'provider', 'skill_curator')`)
	if err != nil {
		t.Fatal(err)
	}
}

func curatorTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return conn
}
