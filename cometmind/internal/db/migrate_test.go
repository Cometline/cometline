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
		subagent_kind TEXT NOT NULL DEFAULT '',
		pinned INTEGER NOT NULL DEFAULT 0,
		context_summary TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
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

func TestEnsureSchemaPreservesExistingSessionsForDisposableSessionCleanup(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v27.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE sessions (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO sessions (id) VALUES ('existing')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 27"); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	var isDisposable int
	if err := conn.QueryRowContext(ctx, `SELECT is_disposable FROM sessions WHERE id = 'existing'`).Scan(&isDisposable); err != nil {
		t.Fatal(err)
	}
	if isDisposable != 0 {
		t.Fatalf("is_disposable = %d, want 0", isDisposable)
	}
}

func TestEnsureSchemaCreatesSessionMediaTable(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v29.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE workspaces (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE sessions (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 28"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d", version, schemaVersion)
	}
	rows, err := conn.QueryContext(ctx, "PRAGMA table_info(session_media)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	for _, col := range []string{"id", "session_id", "storage_session_id", "workspace_id", "kind", "status", "source"} {
		if !columns[col] {
			t.Fatalf("session_media missing column %q after migration", col)
		}
	}
}

func TestEnsureSchemaCopiesStorageSessionID(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v30.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE workspaces (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE sessions (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO workspaces (id) VALUES ('ws1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO sessions (id) VALUES ('sess1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE session_media (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
		workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
		kind TEXT NOT NULL,
		media_type TEXT NOT NULL,
		alt TEXT NOT NULL DEFAULT '',
		prompt TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		provider_id TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT 'generated',
		source_media_id TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'ready',
		byte_size INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER,
		created_at INTEGER NOT NULL DEFAULT 1
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO session_media (
		id, session_id, workspace_id, kind, media_type
	) VALUES ('media1', 'sess1', 'ws1', 'image', 'image/png')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 29"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	var sessionID, storageSessionID string
	if err := conn.QueryRowContext(ctx, `SELECT session_id, storage_session_id FROM session_media WHERE id = 'media1'`).Scan(&sessionID, &storageSessionID); err != nil {
		t.Fatal(err)
	}
	if sessionID != "sess1" || storageSessionID != "sess1" {
		t.Fatalf("session_id=%q storage_session_id=%q", sessionID, storageSessionID)
	}
}

func TestEnsureSchemaV30ReplaysAfterPartialRebuildWithNullSessionID(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v30-replay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	for _, stmt := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces (id) VALUES ('ws1')`,
		`INSERT INTO sessions (id) VALUES ('sess1')`,
		`CREATE TABLE session_media (
			id TEXT PRIMARY KEY,
			session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL,
			storage_session_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			media_type TEXT NOT NULL,
			alt TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			provider_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'generated',
			source_media_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'ready',
			byte_size INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER,
			created_at INTEGER NOT NULL DEFAULT 1
		)`,
		`INSERT INTO session_media (id, session_id, storage_session_id, workspace_id, kind, media_type)
			VALUES ('media-live', 'sess1', 'sess1', 'ws1', 'image', 'image/png')`,
		`INSERT INTO session_media (id, session_id, storage_session_id, workspace_id, kind, media_type, status)
			VALUES ('media-orphan', NULL, 'stored-sess', 'ws1', 'image', 'image/png', 'deleted')`,
		`PRAGMA user_version = 29`,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}

	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d", version, schemaVersion)
	}

	var sessionID sql.NullString
	var storageSessionID string
	if err := conn.QueryRowContext(ctx, `SELECT session_id, storage_session_id FROM session_media WHERE id = 'media-orphan'`).Scan(&sessionID, &storageSessionID); err != nil {
		t.Fatal(err)
	}
	if sessionID.Valid {
		t.Fatalf("session_id = %q, want NULL", sessionID.String)
	}
	if storageSessionID != "stored-sess" {
		t.Fatalf("storage_session_id = %q, want stored-sess", storageSessionID)
	}
}

func TestEnsureSchemaV30CopiesNullSessionIDToMediaID(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v30-null.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	for _, stmt := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces (id) VALUES ('ws1')`,
		`CREATE TABLE session_media (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			workspace_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			media_type TEXT NOT NULL,
			alt TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			provider_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'generated',
			source_media_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'ready',
			byte_size INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER,
			created_at INTEGER NOT NULL DEFAULT 1
		)`,
		`INSERT INTO session_media (id, session_id, workspace_id, kind, media_type)
			VALUES ('media-orphan', NULL, 'ws1', 'image', 'image/png')`,
		`PRAGMA user_version = 29`,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}

	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	var storageSessionID string
	if err := conn.QueryRowContext(ctx, `SELECT storage_session_id FROM session_media WHERE id = 'media-orphan'`).Scan(&storageSessionID); err != nil {
		t.Fatal(err)
	}
	if storageSessionID != "media-orphan" {
		t.Fatalf("storage_session_id = %q, want media-orphan", storageSessionID)
	}
}

func TestRebuildVersionRollsBackWhenAStatementFails(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-rebuild.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE session_media (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		kind TEXT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO session_media (id, session_id, kind) VALUES ('media1', 'sess1', 'image')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 29"); err != nil {
		t.Fatal(err)
	}

	err = applyAlterVersion(ctx, conn, 29, []string{
		"PRAGMA foreign_keys = OFF",
		"DROP TABLE IF EXISTS session_media_new",
		"CREATE TABLE session_media_new (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, kind TEXT NOT NULL)",
		"INSERT INTO session_media_new (id, session_id, kind) SELECT id, session_id, kind FROM session_media",
		"DROP TABLE session_media",
		"SELECT RAISE(ABORT, 'boom')",
		"ALTER TABLE session_media_new RENAME TO session_media",
		"PRAGMA foreign_keys = ON",
	})
	if err == nil {
		t.Fatal("expected rebuild failure")
	}

	var id string
	if scanErr := conn.QueryRowContext(ctx, `SELECT id FROM session_media WHERE id = 'media1'`).Scan(&id); scanErr != nil {
		t.Fatalf("original table should survive a failed rebuild: %v", scanErr)
	}
	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 29 {
		t.Fatalf("user_version = %d, want 29", version)
	}
}

func TestEnsureSchemaV35AddsDetachedMediaTimestamp(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate-v34.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE session_media (
		id TEXT PRIMARY KEY,
		session_id TEXT,
		status TEXT NOT NULL DEFAULT 'ready'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO session_media (id, session_id) VALUES ('media-1', NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA user_version = 34`); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	var detachedAt int64
	if err := conn.QueryRowContext(ctx, `SELECT detached_at FROM session_media WHERE id = 'media-1'`).Scan(&detachedAt); err != nil {
		t.Fatal(err)
	}
	if detachedAt != 0 {
		t.Fatalf("detached_at=%d want 0", detachedAt)
	}
	var version int
	if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version=%d want %d", version, schemaVersion)
	}
}
