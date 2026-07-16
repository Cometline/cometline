// Package diffartifact is the wire contract for edit_file unified diffs.
//
// One module owns markers and formatting so Go tool output and the Cometline
// UI parser cannot drift. The model still sees a plain-text projection;
// structured fields exist for callers that need them without re-parsing.
package diffartifact

import (
	"fmt"
	"strings"
)

// Wire markers — keep in sync with cometline/src/lib/tools/diff-artifact.ts.
const (
	BeginMarker = "*** Begin Diff"
	EndMarker   = "*** End Diff"
)

// Artifact is the deep representation of a successful edit_file diff.
type Artifact struct {
	Path            string
	Added           int
	Deleted         int
	ReplaceAllCount int // 0 when replace_all was not used
	UnifiedDiff     string
}

// Format returns the model-visible / persisted tool output string.
func (a Artifact) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "edited %s (+%d -%d)", a.Path, a.Added, a.Deleted)
	if a.ReplaceAllCount > 0 {
		fmt.Fprintf(&b, " replace_all=%d", a.ReplaceAllCount)
	}
	b.WriteString("\n\n")
	b.WriteString(BeginMarker)
	b.WriteByte('\n')
	diff := a.UnifiedDiff
	b.WriteString(diff)
	if diff != "" && !strings.HasSuffix(diff, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(EndMarker)
	return b.String()
}

// Parse extracts an Artifact from Format output (and legacy identical markers).
func Parse(output string) (Artifact, bool) {
	text := output
	begin := strings.Index(text, BeginMarker)
	end := strings.Index(text, EndMarker)
	if begin < 0 || end < 0 || end <= begin {
		return Artifact{}, false
	}
	summary := strings.TrimSpace(text[:begin])
	body := strings.TrimPrefix(text[begin+len(BeginMarker):end], "\n")
	body = strings.TrimSuffix(body, "\n")
	if strings.TrimSpace(body) == "" {
		return Artifact{}, false
	}
	a := Artifact{UnifiedDiff: body}
	if strings.HasPrefix(summary, "edited ") {
		rest := strings.TrimPrefix(summary, "edited ")
		pathPart, counts, ok := strings.Cut(rest, " (+")
		if ok {
			a.Path = pathPart
			counts = strings.TrimSuffix(counts, ")")
			if before, after, cut := strings.Cut(counts, " replace_all="); cut {
				counts = before
				fmt.Sscanf(after, "%d", &a.ReplaceAllCount)
			}
			var add, del int
			fmt.Sscanf(counts, "%d -%d", &add, &del)
			a.Added, a.Deleted = add, del
		} else {
			a.Path = rest
		}
	}
	return a, true
}
