package gateway

import (
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/jobs"
)

func TestFormatReadyJobsList(t *testing.T) {
	t.Parallel()
	if got := FormatReadyJobsList(nil); got != "No ready jobs." {
		t.Fatalf("empty list = %q", got)
	}
	got := FormatReadyJobsList([]jobs.Job{
		{ID: "hidden-id", Description: "Fix auth", Priority: 2},
		{ID: "hidden-id-2", Description: "Write docs"},
	})
	if strings.Contains(got, "hidden-id") {
		t.Fatalf("list should not include ids: %q", got)
	}
	if !strings.Contains(got, "(p=2) Fix auth") {
		t.Fatalf("missing priority job: %q", got)
	}
	if !strings.Contains(got, "• Write docs") {
		t.Fatalf("missing plain job: %q", got)
	}
}
