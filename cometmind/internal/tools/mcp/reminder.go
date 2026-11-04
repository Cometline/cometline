package mcp

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

// serverFingerprint identifies one connected server's tool surface.
type serverFingerprint struct {
	count uint64
	names uint64
}

// Reminder is the short MCP server list for the system prompt.
// Changed is false when the catalog matches the previous fingerprint.
// Fingerprint is empty when there is nothing to remember.
type Reminder struct {
	Text        string
	Fingerprint string
	Changed     bool
}

// ServerReminder builds the prompt block for connected MCP servers.
// previous is the last fingerprint sent in this chat. Empty means this is the
// first block, so the full list is returned when servers are connected.
func ServerReminder(catalog []mcppkg.CatalogEntry, previous string) Reminder {
	current := encodeFingerprint(fingerprintCatalog(catalog))
	if previous != "" && previous == current {
		return Reminder{Fingerprint: current}
	}
	return Reminder{
		Text:        formatServerReminder(catalog, previous == ""),
		Fingerprint: current,
		Changed:     true,
	}
}

func fingerprintCatalog(catalog []mcppkg.CatalogEntry) map[string]serverFingerprint {
	grouped := map[string][]mcppkg.CatalogEntry{}
	for _, entry := range catalog {
		grouped[entry.ServerID] = append(grouped[entry.ServerID], entry)
	}
	out := make(map[string]serverFingerprint, len(grouped))
	for id, entries := range grouped {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.ToolName)
		}
		sort.Strings(names)
		out[id] = serverFingerprint{
			count: uint64(len(entries)),
			names: hashString(serverLineHash(entries[0].ServerDesc, names)),
		}
	}
	return out
}

func encodeFingerprint(in map[string]serverFingerprint) string {
	if len(in) == 0 {
		return ""
	}
	ids := make([]string, 0, len(in))
	for id := range in {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
	for _, id := range ids {
		fp := in[id]
		fmt.Fprintf(&b, "%s:%d:%d;", id, fp.count, fp.names)
	}
	return b.String()
}

func formatServerReminder(catalog []mcppkg.CatalogEntry, full bool) string {
	if len(catalog) == 0 {
		if full {
			return ""
		}
		return "Connected MCP servers: none."
	}
	grouped := map[string][]mcppkg.CatalogEntry{}
	var order []string
	for _, entry := range catalog {
		if _, ok := grouped[entry.ServerID]; !ok {
			order = append(order, entry.ServerID)
		}
		grouped[entry.ServerID] = append(grouped[entry.ServerID], entry)
	}
	var b strings.Builder
	b.WriteString("Connected MCP servers:\n")
	for _, id := range order {
		entries := grouped[id]
		word := "tools"
		if len(entries) == 1 {
			word = "tool"
		}
		fmt.Fprintf(&b, "- %s", formatServerLine(id, entries, word))
	}
	b.WriteString("MCP tool schemas are not loaded. Use search_mcp_tools, then use_mcp_tool.")
	return b.String()
}

func serverLineHash(desc string, names []string) string {
	return strings.TrimSpace(desc) + "\n" + strings.Join(names, "\n")
}

func formatServerLine(id string, entries []mcppkg.CatalogEntry, word string) string {
	line := fmt.Sprintf("%s (%d %s)", id, len(entries), word)
	desc := ""
	if len(entries) > 0 {
		desc = sanitizeOneLine(entries[0].ServerDesc)
	}
	if desc == "" {
		return line + "\n"
	}
	return line + ": " + desc + "\n"
}

func sanitizeOneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 160
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
