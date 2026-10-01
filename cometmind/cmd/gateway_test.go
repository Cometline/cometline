package cmd

import (
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/jobs"
)

func TestFilterJobsByQuery(t *testing.T) {
	items := []jobs.Job{
		{ID: "job-alpha", Description: "Refactor the router"},
		{ID: "job-beta", Description: "Write release notes"},
	}
	tests := []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"job-alpha", "job-beta"}},
		{query: "  BETA ", want: []string{"job-beta"}},
		{query: "router", want: []string{"job-alpha"}},
		{query: "missing", want: nil},
	}
	for _, tt := range tests {
		got := filterJobsByQuery(items, tt.query)
		if len(got) != len(tt.want) {
			t.Fatalf("filterJobsByQuery(%q) = %d jobs, want %d", tt.query, len(got), len(tt.want))
		}
		for i, job := range got {
			if job.ID != tt.want[i] {
				t.Fatalf("filterJobsByQuery(%q)[%d] = %q, want %q", tt.query, i, job.ID, tt.want[i])
			}
		}
	}
}
