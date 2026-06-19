package memory

import (
	"testing"
)

func TestReciprocalRankFusion(t *testing.T) {
	scores := reciprocalRankFusion(
		[]string{"a", "b", "c"},
		[]string{"b", "d"},
	)
	if scores["b"] <= scores["a"] || scores["b"] <= scores["d"] {
		t.Fatalf("expected b to lead fusion, got %#v", scores)
	}
	if scores["c"] <= 0 || scores["a"] <= 0 {
		t.Fatalf("unexpected scores: %#v", scores)
	}
}

func TestTopNBySimilarity(t *testing.T) {
	got := topNBySimilarity(map[string]float64{
		"a": 0.2,
		"b": 0.9,
		"c": 0.5,
	}, 2)
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("got %v", got)
	}
}
