package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/config"
)

func TestPurgeRuntimeFilesByAge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dir)

	toolDir := filepath.Join(dir, "tool-output")
	tmpDir := filepath.Join(dir, "agent-tmp")
	if err := os.MkdirAll(toolDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		t.Fatal(err)
	}

	oldTool := filepath.Join(toolDir, "old.txt")
	newTool := filepath.Join(toolDir, "new.txt")
	oldTmp := filepath.Join(tmpDir, "old.txt")
	if err := os.WriteFile(oldTool, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newTool, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldTmp, []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldTool, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldTmp, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	to, at := PurgeRuntimeFiles(config.StorageConfig{
		ToolOutputRetentionDays: 7,
		AgentTmpRetentionDays:   3,
	})
	if to != 1 || at != 1 {
		t.Fatalf("deleted tool=%d tmp=%d want 1,1", to, at)
	}
	if _, err := os.Stat(oldTool); !os.IsNotExist(err) {
		t.Fatal("old tool-output should be gone")
	}
	if _, err := os.Stat(newTool); err != nil {
		t.Fatal("new tool-output should remain")
	}
	if _, err := os.Stat(oldTmp); !os.IsNotExist(err) {
		t.Fatal("old agent-tmp should be gone")
	}
}

func TestPurgeRuntimeFilesDoesNotTouchWikiDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dir)

	wikiDir := filepath.Join(dir, "wiki", "entities")
	if err := os.MkdirAll(wikiDir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldWiki := filepath.Join(wikiDir, "stale.md")
	if err := os.WriteFile(oldWiki, []byte("knowledge"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(oldWiki, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	PurgeRuntimeFiles(config.StorageConfig{
		ToolOutputRetentionDays: 1,
		AgentTmpRetentionDays:     1,
	})

	if _, err := os.Stat(oldWiki); err != nil {
		t.Fatalf("wiki file should survive retention purge: %v", err)
	}
}
