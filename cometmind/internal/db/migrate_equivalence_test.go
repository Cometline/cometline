package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// schemaV1 is the oldest user_version CometMind ever stamped. The initial
// CometMind commit shipped schemaVersion 2 with schema.sql already holding
// messages.reasoning_content, and the v1 -> v2 step adds only that column, so
// v1 is that schema without it.
const schemaV1 = `
CREATE TABLE workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    path        TEXT NOT NULL UNIQUE,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id),
    title        TEXT NOT NULL DEFAULT '',
    model_id     TEXT NOT NULL,
    provider_id  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'archived')),
    token_usage  TEXT NOT NULL DEFAULT '{}',
    created_at   INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at   INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE messages (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    role        TEXT NOT NULL
                CHECK (
                    role IN ('user', 'assistant', 'tool_result', 'system')
                ),
    content     TEXT NOT NULL DEFAULT '',
    token_count INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE tool_calls (
    id          TEXT PRIMARY KEY,
    message_id  TEXT NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    tool_name   TEXT NOT NULL,
    arguments   TEXT NOT NULL DEFAULT '{}',
    result      TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    exit_code   INTEGER,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX idx_sessions_workspace ON sessions (workspace_id);

CREATE INDEX idx_sessions_updated ON sessions (updated_at DESC);

CREATE INDEX idx_messages_session ON messages (session_id, created_at);

CREATE INDEX idx_tool_calls_message ON tool_calls (message_id);

INSERT INTO workspaces (id, name, path) VALUES ('ws1', 'ws', '/tmp/ws');

INSERT INTO sessions (id, workspace_id, model_id, provider_id) VALUES ('s1', 'ws1', 'm', 'p');

INSERT INTO messages (id, session_id, role, content) VALUES ('m1', 's1', 'user', 'hi');

INSERT INTO tool_calls (id, message_id, tool_name) VALUES ('t1', 'm1', 'read');

PRAGMA user_version = 1;
`

func TestMigrationsFromV1MatchFreshSchema(t *testing.T) {
	ctx := context.Background()

	fresh := openTestDB(t, "fresh.db")
	if err := EnsureSchema(ctx, fresh); err != nil {
		t.Fatalf("EnsureSchema(fresh) error = %v", err)
	}

	upgraded := openTestDB(t, "v1.db")
	for _, stmt := range splitStatements(schemaV1) {
		if _, err := upgraded.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("build v1: %v\nstatement: %s", err, stmt)
		}
	}
	if err := EnsureSchema(ctx, upgraded); err != nil {
		t.Fatalf("EnsureSchema(v1) error = %v", err)
	}

	for _, conn := range []*sql.DB{fresh, upgraded} {
		var version int
		if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version != schemaVersion {
			t.Fatalf("user_version = %d, want %d", version, schemaVersion)
		}
	}
	var content string
	if err := upgraded.QueryRowContext(ctx, `SELECT content FROM messages WHERE id = 'm1'`).Scan(&content); err != nil {
		t.Fatalf("v1 message lost during upgrade: %v", err)
	}

	want := describeSchema(t, fresh)
	got := describeSchema(t, upgraded)
	missing, extra := diffSorted(want, got)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("upgraded schema drifts from schema.sql\nonly in schema.sql:\n  %s\nonly in upgraded DB:\n  %s",
			strings.Join(missing, "\n  "), strings.Join(extra, "\n  "))
	}
}

func openTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// describeSchema flattens the catalog into sorted, whitespace-normalized
// lines so two databases compare by meaning rather than by DDL text or
// column order.
func describeSchema(t *testing.T, conn *sql.DB) []string {
	t.Helper()
	var lines []string
	type object struct{ kind, name, table, sql string }
	var objects []object
	rows, err := conn.Query(`SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master
		WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var o object
		if err := rows.Scan(&o.kind, &o.name, &o.table, &o.sql); err != nil {
			t.Fatal(err)
		}
		objects = append(objects, o)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	for _, o := range objects {
		if o.kind != "table" {
			continue
		}
		lines = append(lines, "table "+o.name+virtualModule(o.sql))
		lines = append(lines, describeColumns(t, conn, o.name)...)
		lines = append(lines, describeForeignKeys(t, conn, o.name)...)
		lines = append(lines, describeIndexes(t, conn, o.name)...)
		for _, check := range checkConstraints(o.sql) {
			lines = append(lines, fmt.Sprintf("check %s %s", o.name, check))
		}
	}
	slices.Sort(lines)
	return lines
}

func virtualModule(createSQL string) string {
	_, module, ok := strings.Cut(createSQL, " USING ")
	if !strings.HasPrefix(strings.ToUpper(createSQL), "CREATE VIRTUAL TABLE") || !ok {
		return ""
	}
	return " using " + normalizeSQL(module)
}

func describeColumns(t *testing.T, conn *sql.DB, table string) []string {
	t.Helper()
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		def := "<none>"
		if dflt.Valid {
			def = normalizeSQL(dflt.String)
		}
		lines = append(lines, fmt.Sprintf("column %s.%s type=%s notnull=%d default=%s pk=%d",
			table, name, strings.ToUpper(columnType), notNull, def, pk))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func describeForeignKeys(t *testing.T, conn *sql.DB, table string) []string {
	t.Helper()
	rows, err := conn.Query(fmt.Sprintf("PRAGMA foreign_key_list(%q)", table))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var id, seq int
		var parent, from, onUpdate, onDelete, match string
		var to sql.NullString
		if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, fmt.Sprintf("fk %s.%s -> %s(%s) on_update=%s on_delete=%s match=%s",
			table, from, parent, to.String, onUpdate, onDelete, match))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func describeIndexes(t *testing.T, conn *sql.DB, table string) []string {
	t.Helper()
	type index struct {
		name, origin    string
		unique, partial int
	}
	rows, err := conn.Query(fmt.Sprintf("PRAGMA index_list(%q)", table))
	if err != nil {
		t.Fatal(err)
	}
	var indexes []index
	for rows.Next() {
		var seq int
		var ix index
		if err := rows.Scan(&seq, &ix.name, &ix.unique, &ix.origin, &ix.partial); err != nil {
			t.Fatal(err)
		}
		indexes = append(indexes, ix)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	var lines []string
	for _, ix := range indexes {
		cols := indexColumns(t, conn, ix.name)
		// Implicit UNIQUE/PRIMARY KEY indexes are auto-named, so identify them
		// by their columns instead.
		name := ix.name
		where := ""
		if ix.origin != "c" {
			name = "<auto>"
		} else {
			var createSQL string
			if err := conn.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, ix.name).Scan(&createSQL); err != nil {
				t.Fatal(err)
			}
			if _, clause, ok := cutFold(normalizeSQL(createSQL), " WHERE "); ok {
				where = " where=" + clause
			}
		}
		lines = append(lines, fmt.Sprintf("index %s.%s origin=%s unique=%d partial=%d cols=%s%s",
			table, name, ix.origin, ix.unique, ix.partial, strings.Join(cols, ","), where))
	}
	return lines
}

func indexColumns(t *testing.T, conn *sql.DB, index string) []string {
	t.Helper()
	rows, err := conn.Query(fmt.Sprintf("PRAGMA index_xinfo(%q)", index))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var seqno, cid, desc, key int
		var name sql.NullString
		var coll string
		if err := rows.Scan(&seqno, &cid, &name, &desc, &coll, &key); err != nil {
			t.Fatal(err)
		}
		if key == 0 {
			continue
		}
		dir := "ASC"
		if desc == 1 {
			dir = "DESC"
		}
		cols = append(cols, fmt.Sprintf("%s %s %s", name.String, dir, coll))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return cols
}

var checkKeyword = regexp.MustCompile(`(?i)\bCHECK\s*\(`)

// checkConstraints returns every CHECK (...) body in a CREATE TABLE statement.
// PRAGMA table_info does not expose them, and ALTER TABLE ADD COLUMN appends
// its column text (CHECK included) to the stored statement.
func checkConstraints(createSQL string) []string {
	var out []string
	for _, loc := range checkKeyword.FindAllStringIndex(createSQL, -1) {
		start := loc[1] - 1
		depth := 0
		for i := start; i < len(createSQL); i++ {
			switch createSQL[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				out = append(out, normalizeSQL(createSQL[start:i+1]))
				break
			}
		}
	}
	slices.Sort(out)
	return out
}

var (
	whitespaceRun = regexp.MustCompile(`\s+`)
	punctSpacing  = regexp.MustCompile(` ?([(),*=]) ?`)
)

func normalizeSQL(s string) string {
	s = whitespaceRun.ReplaceAllString(strings.TrimSpace(s), " ")
	return punctSpacing.ReplaceAllString(s, "$1")
}

func cutFold(s, sep string) (before, after string, found bool) {
	if i := strings.Index(strings.ToUpper(s), strings.ToUpper(sep)); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

func diffSorted(want, got []string) (missing, extra []string) {
	i, j := 0, 0
	for i < len(want) || j < len(got) {
		switch {
		case j == len(got) || (i < len(want) && want[i] < got[j]):
			missing = append(missing, want[i])
			i++
		case i == len(want) || got[j] < want[i]:
			extra = append(extra, got[j])
			j++
		default:
			i++
			j++
		}
	}
	return missing, extra
}
