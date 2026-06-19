package memory

import (
	"strings"
	"unicode"
)

// buildFTSMatchQuery tokenizes free text into a safe FTS5 OR expression.
// Returns empty string when no usable tokens are found.
func buildFTSMatchQuery(query string) string {
	tokens := tokenizeFTS(query)
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, len(tokens))
	for i, tok := range tokens {
		parts[i] = `"` + strings.ReplaceAll(tok, `"`, `""`) + `"`
	}
	return strings.Join(parts, " OR ")
}

func tokenizeFTS(query string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, field := range strings.FieldsFunc(query, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == '/')
	}) {
		field = strings.Trim(field, "._-/")
		if len(field) < 2 {
			continue
		}
		lower := strings.ToLower(field)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, lower)
	}
	return out
}
