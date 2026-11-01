package catalog

import (
	"testing"
)

func TestListIncludesLockedEntries(t *testing.T) {
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 4 {
		t.Fatalf("expected at least 4 catalog entries, got %d", len(list))
	}
	ids := map[string]bool{}
	for _, e := range list {
		ids[e.ID] = true
	}
	for _, want := range []string{"atlassian", "heptabase", "linear", "sentry"} {
		if !ids[want] {
			t.Fatalf("missing catalog entry %q", want)
		}
	}
}

func TestMaterializeAtlassianUsesNormalizedURL(t *testing.T) {
	entry, ok, err := ByID("atlassian")
	if err != nil || !ok {
		t.Fatalf("atlassian entry: ok=%v err=%v", ok, err)
	}
	got := Materialize(entry, nil)
	if got.Source != "catalog" || got.CatalogID != "atlassian" {
		t.Fatalf("catalog metadata: %+v", got)
	}
	if got.URL != "https://mcp.atlassian.com/v2/mcp" {
		t.Fatalf("url = %q", got.URL)
	}
}

func TestHostKeyDedupesByHostAndPath(t *testing.T) {
	a := HostKey("https://api.heptabase.com/mcp")
	b := HostKey("https://api.heptabase.com/mcp/")
	if a == "" || a != b {
		t.Fatalf("host keys: %q vs %q", a, b)
	}
	if HostKey("https://mcp.linear.app/mcp") == HostKey("https://mcp.linear.app/mcp/readonly") {
		t.Fatal("expected different host keys for linear endpoints")
	}
}
