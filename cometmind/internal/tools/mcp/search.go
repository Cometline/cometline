package mcp

import (
	"encoding/json"
	"strings"
	"unicode"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

const (
	defaultSearchLimit = 5
	maxSearchLimit     = 8
	maxDescriptionLen  = 2048
)

// searchHit is one ranked catalog match. Schema stays on the entry so the
// model can call the tool without a second lookup.
type searchHit struct {
	entry mcppkg.CatalogEntry
	score int
}

// searchCatalog returns an exact name match first. Otherwise it ranks by
// keyword overlap over server, tool name, description, and parameter names.
func searchCatalog(entries []mcppkg.CatalogEntry, query string, limit int) []searchHit {
	query = strings.TrimSpace(query)
	if query == "" || len(entries) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	if hits := exactHits(entries, query); len(hits) > 0 {
		if len(hits) > limit {
			return hits[:limit]
		}
		return hits
	}

	terms := queryTerms(query)
	if len(terms) == 0 {
		return nil
	}
	hits := make([]searchHit, 0, len(entries))
	for _, entry := range entries {
		score := scoreEntry(entry, terms)
		if score == 0 {
			continue
		}
		hits = append(hits, searchHit{entry: entry, score: score})
	}
	sortHits(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func exactHits(entries []mcppkg.CatalogEntry, query string) []searchHit {
	q := strings.ToLower(query)
	var qualified, bare []searchHit
	for _, entry := range entries {
		switch {
		case strings.ToLower(entry.RegistryName) == q:
			qualified = append(qualified, searchHit{entry: entry, score: 1})
		case strings.ToLower(entry.ToolName) == q:
			bare = append(bare, searchHit{entry: entry, score: 1})
		}
	}
	if len(qualified) > 0 {
		return qualified
	}
	return bare
}

func scoreEntry(entry mcppkg.CatalogEntry, terms []string) int {
	doc := strings.ToLower(searchDocument(entry))
	score := 0
	for _, term := range terms {
		if strings.Contains(doc, term) {
			score++
		}
	}
	return score
}

func searchDocument(entry mcppkg.CatalogEntry) string {
	parts := []string{
		entry.ServerName,
		entry.ServerID,
		entry.ToolName,
		entry.RegistryName,
		entry.Description,
	}
	parts = append(parts, parameterNames(entry.Parameters)...)
	var b strings.Builder
	for _, part := range parts {
		for _, word := range splitIdent(part) {
			b.WriteString(word)
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func queryTerms(query string) []string {
	seen := map[string]struct{}{}
	var terms []string
	for _, field := range strings.Fields(query) {
		for _, word := range splitIdent(field) {
			word = strings.ToLower(word)
			if len(word) < 2 {
				continue
			}
			if _, ok := seen[word]; ok {
				continue
			}
			seen[word] = struct{}{}
			terms = append(terms, word)
		}
	}
	return terms
}

// splitIdent breaks snake_case, kebab-case, and camelCase into words.
func splitIdent(s string) []string {
	var words []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	}) {
		words = append(words, splitCamel(part)...)
	}
	return words
}

func splitCamel(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	start := 0
	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

func sortHits(hits []searchHit) {
	for i := 1; i < len(hits); i++ {
		item := hits[i]
		j := i
		for j > 0 && worseHit(hits[j-1], item) {
			hits[j] = hits[j-1]
			j--
		}
		hits[j] = item
	}
}

func worseHit(a, b searchHit) bool {
	if a.score != b.score {
		return a.score < b.score
	}
	return a.entry.RegistryName > b.entry.RegistryName
}

func parameterNames(schema json.RawMessage) []string {
	if len(schema) == 0 {
		return nil
	}
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return nil
	}
	names := make([]string, 0, len(doc.Properties))
	for name := range doc.Properties {
		names = append(names, name)
	}
	return names
}

func truncateDescription(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxDescriptionLen {
		return s
	}
	return string(runes[:maxDescriptionLen]) + "…"
}
