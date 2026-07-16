package diffartifact

import (
	"strings"
	"testing"
)

func TestFormatParseRoundTrip(t *testing.T) {
	a := Artifact{
		Path:            "pkg/foo.go",
		Added:           2,
		Deleted:         1,
		ReplaceAllCount: 3,
		UnifiedDiff:     "--- a/pkg/foo.go\n+++ b/pkg/foo.go\n@@ -1,1 +1,2 @@\n-old\n+new\n+line\n",
	}
	out := a.Format()
	if !strings.Contains(out, BeginMarker) || !strings.Contains(out, EndMarker) {
		t.Fatalf("markers missing: %q", out)
	}
	got, ok := Parse(out)
	if !ok {
		t.Fatal("Parse failed")
	}
	if got.Path != a.Path || got.Added != a.Added || got.Deleted != a.Deleted || got.ReplaceAllCount != a.ReplaceAllCount {
		t.Fatalf("got %+v want %+v", got, a)
	}
	wantBody := strings.TrimSuffix(a.UnifiedDiff, "\n")
	if got.UnifiedDiff != wantBody {
		t.Fatalf("diff body = %q want %q", got.UnifiedDiff, wantBody)
	}
}

func TestParseRejectsMissingMarkers(t *testing.T) {
	if _, ok := Parse("edited x (+1 -0)\nno markers"); ok {
		t.Fatal("expected false")
	}
}
