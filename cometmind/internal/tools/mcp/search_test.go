package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

func testCatalog() []mcppkg.CatalogEntry {
	return []mcppkg.CatalogEntry{
		{
			ServerID:     "svelte",
			ServerName:   "Svelte",
			ToolName:     "get-documentation",
			RegistryName: "mcp_svelte_get-documentation",
			Description:  "Read a Svelte documentation section",
			Parameters:   json.RawMessage(`{"type":"object","properties":{"section":{"type":"string"}}}`),
		},
		{
			ServerID:     "gitnexus",
			ServerName:   "GitNexus",
			ToolName:     "query",
			RegistryName: "mcp_gitnexus_query",
			Description:  "Search the code graph",
			Parameters:   json.RawMessage(`{"type":"object","properties":{"search_query":{"type":"string"}}}`),
		},
		{
			ServerID:     "svelte",
			ServerName:   "Svelte",
			ToolName:     "list-sections",
			RegistryName: "mcp_svelte_list-sections",
			Description:  "List documentation sections",
		},
	}
}

func TestSearchCatalogExactNameWins(t *testing.T) {
	hits := searchCatalog(testCatalog(), "mcp_gitnexus_query", 5)
	if len(hits) != 1 || hits[0].entry.ToolName != "query" {
		t.Fatalf("exact hit = %#v", hits)
	}
}

func TestSearchCatalogRanksKeywords(t *testing.T) {
	hits := searchCatalog(testCatalog(), "svelte documentation", 5)
	if len(hits) == 0 || hits[0].entry.ServerID != "svelte" {
		t.Fatalf("first hit = %#v, want svelte", hits)
	}
	if strings.Contains(hits[0].entry.RegistryName, "gitnexus") {
		t.Fatalf("gitnexus ranked first for a svelte query")
	}
}

func TestSearchCatalogSplitsIdentifiers(t *testing.T) {
	hits := searchCatalog(testCatalog(), "get documentation", 5)
	if len(hits) == 0 || hits[0].entry.ToolName != "get-documentation" {
		t.Fatalf("split ident hit = %#v", hits)
	}
}

func TestSearchCatalogBareNameReturnsEveryServer(t *testing.T) {
	catalog := append(testCatalog(), mcppkg.CatalogEntry{
		ServerID:     "other",
		ToolName:     "query",
		RegistryName: "mcp_other_query",
		Description:  "Another query tool",
	})
	hits := searchCatalog(catalog, "query", 5)
	if len(hits) != 2 {
		t.Fatalf("bare query hits = %d, want 2", len(hits))
	}
}

func TestFindCatalogEntryRejectsAmbiguousBareName(t *testing.T) {
	catalog := []mcppkg.CatalogEntry{
		{ServerID: "a", ToolName: "query", RegistryName: "mcp_a_query"},
		{ServerID: "b", ToolName: "query", RegistryName: "mcp_b_query"},
	}
	if _, ok := findCatalogEntry(catalog, "query"); ok {
		t.Fatal("ambiguous bare name should not resolve")
	}
	got, ok := findCatalogEntry(catalog, "mcp_b_query")
	if !ok || got.ServerID != "b" {
		t.Fatalf("qualified lookup = %#v ok=%v", got, ok)
	}
}

func TestSearchCatalogEmptyQuery(t *testing.T) {
	if hits := searchCatalog(testCatalog(), "  ", 5); hits != nil {
		t.Fatalf("empty query returned %#v", hits)
	}
}

func TestServerReminderIncludesDescription(t *testing.T) {
	catalog := testCatalog()
	catalog[0].ServerDesc = "Svelte and SvelteKit docs"
	catalog[2].ServerDesc = "Svelte and SvelteKit docs"
	got := ServerReminder(catalog, "")
	if !strings.Contains(got.Text, "svelte (2 tools): Svelte and SvelteKit docs") {
		t.Fatalf("reminder = %q", got.Text)
	}
}

func TestServerReminderSkipsUnchangedCatalog(t *testing.T) {
	catalog := testCatalog()
	first := ServerReminder(catalog, "")
	if !first.Changed || !strings.Contains(first.Text, "svelte (2 tools)") {
		t.Fatalf("first reminder = %#v", first)
	}
	again := ServerReminder(catalog, first.Fingerprint)
	if again.Changed || again.Text != "" {
		t.Fatalf("unchanged reminder = %#v", again)
	}
}

func TestServerReminderReportsDisconnect(t *testing.T) {
	first := ServerReminder(testCatalog(), "")
	gone := ServerReminder(nil, first.Fingerprint)
	if !gone.Changed || !strings.Contains(gone.Text, "none") {
		t.Fatalf("disconnect reminder = %#v", gone)
	}
}

func TestFormatSearchResultIncludesSchema(t *testing.T) {
	catalog := testCatalog()
	hits := searchCatalog(catalog, "mcp_svelte_get-documentation", 5)
	out := formatSearchResult(catalog, hits)
	for _, want := range []string{`"name": "mcp_svelte_get-documentation"`, `"section"`, `"total_hidden_tools": 2`} {
		if !strings.Contains(out, want) {
			t.Fatalf("result missing %q:\n%s", want, out)
		}
	}
}
