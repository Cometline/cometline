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

func TestSyncLockedServerUpdatesRecipeWhenCatalogVersionChanges(t *testing.T) {
	res := SyncLockedServer(SyncLockedServerInput{
		Fields: ServerCatalogFields{
			Source:         ServerSourceCatalog,
			CatalogID:      "atlassian",
			CatalogVersion: "0",
		},
		Current: LockedServerRecipe{
			Name:           "Old",
			Transport:      "http",
			URL:            "https://mcp.atlassian.com/v1/mcp",
			CatalogVersion: "0",
		},
	})
	if !res.Changed {
		t.Fatal("expected catalog version bump to change recipe")
	}
	if res.Next.URL != "https://mcp.atlassian.com/v2/mcp" {
		t.Fatalf("url = %q", res.Next.URL)
	}
	if res.Next.CatalogVersion != "1" {
		t.Fatalf("catalogVersion = %q", res.Next.CatalogVersion)
	}
}

func TestHostKeyDedupesByHostAndPath(t *testing.T) {
	a := HostKey("https://api.heptabase.com/mcp")
	b := HostKey("https://api.heptabase.com/mcp/")
	if a == "" || a != b {
		t.Fatalf("host keys: %q vs %q", a, b)
	}
	if HostKey("https://mcp.atlassian.com/v1/mcp") != "mcp.atlassian.com" {
		t.Fatalf("atlassian v1 host key")
	}
	if HostKey("https://mcp.atlassian.com/v2/mcp") != "mcp.atlassian.com" {
		t.Fatalf("atlassian v2 host key")
	}
	if HostKey("https://mcp.linear.app/mcp") == HostKey("https://mcp.linear.app/mcp/readonly") {
		t.Fatal("expected different host keys for linear endpoints")
	}
}
