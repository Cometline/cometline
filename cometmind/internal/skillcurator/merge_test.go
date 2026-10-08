package skillcurator

import "testing"

func TestMergeSources(t *testing.T) {
	got := MergeSources("keep", []string{"keep", "old", "pinned", "old"}, true, map[string]bool{"pinned": true})
	if len(got) != 1 || got[0] != "old" {
		t.Fatalf("sources = %#v", got)
	}
	if MergeSources("keep", []string{"old"}, false, nil) != nil {
		t.Fatal("unchanged target must not archive sources")
	}
}
