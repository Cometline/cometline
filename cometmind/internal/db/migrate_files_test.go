package db

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedMigrationsAreContiguousUpToSchemaVersion(t *testing.T) {
	if len(migrations) == 0 {
		t.Fatal("no embedded migrations")
	}
	for i, m := range migrations {
		if want := i + 2; m.version != want {
			t.Fatalf("migrations[%d] = %s, want version %04d", i, m.name, want)
		}
	}
	if last := migrations[len(migrations)-1].version; last != schemaVersion {
		t.Fatalf("last migration version = %d, schemaVersion = %d", last, schemaVersion)
	}
}

func TestLoadMigrationsSplitsStatementsAndSkipsComments(t *testing.T) {
	got, err := loadMigrations(fstest.MapFS{
		"0002_first.sql":  {Data: []byte("-- header; with a semicolon\nCREATE TABLE a (id TEXT);\n\n-- note\nCREATE INDEX i ON a (id);\n")},
		"0003_second.sql": {Data: []byte("ALTER TABLE a ADD COLUMN b TEXT;")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].version != 2 || got[1].version != 3 {
		t.Fatalf("versions = %+v", got)
	}
	want := []string{"CREATE TABLE a (id TEXT);", "CREATE INDEX i ON a (id);"}
	if !slices.Equal(got[0].stmts, want) {
		t.Fatalf("stmts = %q, want %q", got[0].stmts, want)
	}
}

func TestLoadMigrationsRejectsMalformedFiles(t *testing.T) {
	ok := &fstest.MapFile{Data: []byte("SELECT 1;")}
	for name, tc := range map[string]struct {
		files   fstest.MapFS
		wantErr string
	}{
		"gap":          {fstest.MapFS{"0002_a.sql": ok, "0004_b.sql": ok}, "want version 0003"},
		"not from 2":   {fstest.MapFS{"0001_a.sql": ok}, "want version 0002"},
		"bad name":     {fstest.MapFS{"0002-a.sql": ok}, "NNNN_description.sql"},
		"unterminated": {fstest.MapFS{"0002_a.sql": {Data: []byte("SELECT 1;\nSELECT 2")}}, "terminating ';'"},
		"empty":        {fstest.MapFS{"0002_a.sql": {Data: []byte("-- nothing\n")}}, "no statements"},
		"no files":     {fstest.MapFS{}, "no migration files"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadMigrations(tc.files)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
