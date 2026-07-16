package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundToolOutputSmallUnchanged(t *testing.T) {
	in := "hello"
	if got := boundToolOutput(in); got != in {
		t.Fatalf("got %q", got)
	}
}

func TestBoundToolOutputSpillsLarge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dir)

	var b strings.Builder
	for i := 0; i < toolOutputMaxPreviewRunes+5000; i++ {
		b.WriteByte('a')
	}
	out := boundToolOutput(b.String())
	if !strings.Contains(out, "output truncated") {
		t.Fatalf("expected truncation notice: %s", out[:min(200, len(out))])
	}
	if !strings.Contains(out, "tool-output") {
		t.Fatalf("expected spill path: %s", out[len(out)-200:])
	}
	// Find the spill file.
	spillDir := filepath.Join(dir, "tool-output")
	entries, err := os.ReadDir(spillDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("spill files = %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(spillDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != b.Len() {
		t.Fatalf("spill len = %d, want %d", len(data), b.Len())
	}
}

func TestHeadTailPreview(t *testing.T) {
	in := strings.Repeat("a", 100) + "MID" + strings.Repeat("b", 100)
	got := headTailPreview(in, 10, 10)
	if !strings.Contains(got, "omitted") {
		t.Fatalf("got %q", got)
	}
	if !strings.HasPrefix(got, "aaaaaaaaaa") {
		t.Fatalf("head: %q", got)
	}
	if !strings.HasSuffix(got, "bbbbbbbbbb") {
		t.Fatalf("tail: %q", got)
	}
}

func TestRunCommandTimeoutParam(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(t.Context(), []byte(`{"command":"sleep 2","timeout_sec":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "timed out") {
		t.Fatalf("result = %+v", res)
	}
}
