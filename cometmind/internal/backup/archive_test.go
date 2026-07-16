package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRunCreatesZipAndRotatesOldBackups(t *testing.T) {
	dataDir := t.TempDir()
	destDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)

	if err := os.WriteFile(filepath.Join(dataDir, "cometline-settings.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	wikiDir := filepath.Join(dataDir, "wiki", "entities")
	if err := os.MkdirAll(wikiDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wikiDir, "test.md"), []byte("# wiki"), 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dataDir, "cometmind.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO notes (body) VALUES ('persist')`); err != nil {
		t.Fatal(err)
	}

	archiver := &Archiver{DB: db}
	for i := 0; i < 3; i++ {
		res, err := archiver.Run(context.Background(), Config{
			DestinationDir: destDir,
			MaxBackups:     2,
		})
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if !strings.HasPrefix(filepath.Base(res.Path), backupNamePrefix) {
			t.Fatalf("unexpected backup name %q", res.Path)
		}
		if res.FilesZipped == 0 {
			t.Fatalf("expected files in zip")
		}
		time.Sleep(1100 * time.Millisecond)
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	var zips []string
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".zip") {
			zips = append(zips, ent.Name())
		}
	}
	if len(zips) != 2 {
		t.Fatalf("rotation: got %d zips want 2: %v", len(zips), zips)
	}

	latest := filepath.Join(destDir, zips[len(zips)-1])
	r, err := zip.OpenReader(latest)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	foundSettings := false
	foundWiki := false
	for _, f := range r.File {
		switch f.Name {
		case "cometline-settings.json":
			foundSettings = true
		case "wiki/entities/test.md":
			foundWiki = true
		}
	}
	if !foundSettings || !foundWiki {
		t.Fatalf("zip missing expected entries; settings=%v wiki=%v", foundSettings, foundWiki)
	}
}

func TestRunRejectsDestinationInsideDataDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	nested := filepath.Join(dataDir, "backups")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := (&Archiver{}).Run(context.Background(), Config{DestinationDir: nested})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected outside error, got %v", err)
	}
}
