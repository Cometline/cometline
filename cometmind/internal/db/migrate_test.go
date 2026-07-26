package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEnsureSchemaV26MigratesMemoryPoliciesAndIsolatesLegacyOutcomes(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v25.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE memories (
		id TEXT PRIMARY KEY, scope TEXT NOT NULL DEFAULT 'global', kind TEXT NOT NULL DEFAULT 'fact',
		preference_category TEXT NOT NULL DEFAULT '', content TEXT NOT NULL, embedding BLOB,
		embedding_model TEXT, source TEXT NOT NULL, base_weight REAL NOT NULL DEFAULT 1,
		access_count INTEGER NOT NULL DEFAULT 0, pinned INTEGER NOT NULL DEFAULT 0,
		source_session_id TEXT, superseded_by TEXT, archived INTEGER NOT NULL DEFAULT 0,
		archived_reason TEXT, last_accessed_at INTEGER, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id, kind string
		pinned   int
	}{
		{"pinned-pref", "preference", 1}, {"plain-pref", "preference", 0},
		{"pinned-fact", "fact", 1}, {"legacy-outcome", "task_outcome", 0},
	} {
		if _, err := conn.ExecContext(ctx, `INSERT INTO memories (id, kind, content, source, pinned, created_at, updated_at) VALUES (?, ?, ?, 'test', ?, 1, 1)`, row.id, row.kind, row.id, row.pinned); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 25"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	checks := map[string][4]string{}
	rows, err := conn.QueryContext(ctx, `SELECT id, application_policy, retention_policy, origin_type, origin_id FROM memories`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, app, retention, originType, originID string
		if err := rows.Scan(&id, &app, &retention, &originType, &originID); err != nil {
			t.Fatal(err)
		}
		checks[id] = [4]string{app, retention, originType, originID}
	}
	if got := checks["pinned-pref"]; got[0] != "always" || got[1] != "protected" {
		t.Fatalf("pinned preference = %#v", got)
	}
	if got := checks["plain-pref"]; got[0] != "relevant" || got[1] != "decaying" {
		t.Fatalf("plain preference = %#v", got)
	}
	if got := checks["pinned-fact"]; got[0] != "relevant" || got[1] != "protected" {
		t.Fatalf("pinned fact = %#v", got)
	}
	if got := checks["legacy-outcome"]; got[2] != "legacy" || got[3] != "legacy:legacy-outcome" {
		t.Fatalf("legacy outcome lineage = %#v", got)
	}
	columns, err := conn.QueryContext(ctx, `PRAGMA table_info(memories)`)
	if err != nil {
		t.Fatal(err)
	}
	defer columns.Close()
	for columns.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := columns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "pinned" {
			t.Fatal("legacy memory pinned column should be removed")
		}
	}
}

func TestEnsureSchemaUpgradesContextSummaryColumns(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "migrate-v9.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 9"); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		model_id TEXT NOT NULL,
		provider_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		token_usage TEXT NOT NULL DEFAULT '{}',
		parent_session_id TEXT,
		purpose TEXT NOT NULL DEFAULT '',
		delegation_status TEXT NOT NULL DEFAULT '',
		output_summary TEXT NOT NULL DEFAULT '',
		acp_session_id TEXT NOT NULL DEFAULT '',
		pending_question TEXT NOT NULL DEFAULT '',
		pinned INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create sessions table: %v", err)
	}

	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d", version, schemaVersion)
	}

	columns := map[string]bool{}
	rows, err := conn.QueryContext(ctx, "PRAGMA table_info(sessions)")
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		columns[name] = true
	}
	for _, col := range []string{"context_summary", "compacted_until_message_id", "context_summary_updated_at"} {
		if !columns[col] {
			t.Fatalf("sessions missing column %q after migration", col)
		}
	}
}
