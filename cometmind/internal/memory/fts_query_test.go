package memory

import "testing"

func TestBuildFTSMatchQuery(t *testing.T) {
	got := buildFTSMatchQuery("cometmind retriever.go SvelteKit")
	if got == "" {
		t.Fatal("expected tokens")
	}
	if got != `"cometmind" OR "retriever.go" OR "sveltekit"` {
		t.Fatalf("got %q", got)
	}
}

func TestBuildFTSMatchQuery_Empty(t *testing.T) {
	if buildFTSMatchQuery("!!!") != "" {
		t.Fatal("expected empty for punctuation-only query")
	}
}

func TestTokenizeFTSDedupes(t *testing.T) {
	tokens := tokenizeFTS("CometMind cometmind COMETMIND")
	if len(tokens) != 1 {
		t.Fatalf("got %v", tokens)
	}
}
