package db

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

//go:embed schema.sql
var schemaSQL string

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migration upgrades a database from user_version-1 to version.
type migration struct {
	version int
	name    string
	stmts   []string
}

var migrations = mustLoadMigrations()

// schemaVersion is the user_version of a fully migrated database: the number
// of the last file in migrations/.
var schemaVersion = migrations[len(migrations)-1].version

var migrationFileName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

func mustLoadMigrations() []migration {
	dir, err := fs.Sub(migrationFiles, "migrations")
	if err == nil {
		var loaded []migration
		if loaded, err = loadMigrations(dir); err == nil {
			return loaded
		}
	}
	panic(fmt.Sprintf("db: load embedded migrations: %v", err))
}

// loadMigrations reads NNNN_description.sql files in version order. Versions
// must be contiguous from 0002 because user_version 1 is the oldest baseline.
func loadMigrations(dir fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, entry := range entries {
		match := migrationFileName.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("%s: name must match NNNN_description.sql", entry.Name())
		}
		version, _ := strconv.Atoi(match[1])
		if want := len(out) + 2; version != want {
			return nil, fmt.Errorf("%s: want version %04d, versions must be contiguous from 0002", entry.Name(), want)
		}
		body, err := fs.ReadFile(dir, entry.Name())
		if err != nil {
			return nil, err
		}
		src := stripLineComments(string(body))
		if tail := src[strings.LastIndex(src, ";")+1:]; strings.TrimSpace(tail) != "" {
			return nil, fmt.Errorf("%s: last statement is missing its terminating ';'", entry.Name())
		}
		stmts := splitStatements(src)
		if len(stmts) == 0 {
			return nil, fmt.Errorf("%s: no statements", entry.Name())
		}
		out = append(out, migration{version: version, name: entry.Name(), stmts: stmts})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no migration files")
	}
	return out, nil
}

func stripLineComments(src string) string {
	lines := strings.Split(src, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func isForeignKeysPragma(stmt string) bool {
	normalized := strings.ToLower(strings.TrimSpace(stmt))
	normalized = strings.TrimSuffix(normalized, ";")
	return strings.HasPrefix(normalized, "pragma foreign_keys")
}

func isTableRebuild(stmts []string) bool {
	for _, stmt := range stmts {
		if strings.Contains(strings.ToUpper(stmt), "DROP TABLE") {
			return true
		}
	}
	return false
}

// splitStatements splits SQL on ';'. Statements must not contain ';' inside
// string literals, and comments are only allowed on their own line.
func splitStatements(src string) []string {
	var out []string
	for _, part := range strings.Split(stripLineComments(src), ";") {
		if stmt := strings.TrimSpace(part); stmt != "" {
			out = append(out, stmt+";")
		}
	}
	return out
}
