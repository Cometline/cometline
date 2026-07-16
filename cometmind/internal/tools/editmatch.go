package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// applySearchReplace replaces oldString with newString in content using an
// OpenCode-inspired exact-then-fuzzy matching pipeline.
//
// Returns the new content and the actual matched span that was replaced
// (which may differ from oldString when a fuzzy replacer was used).
func applySearchReplace(content, oldString, newString string, replaceAll bool) (next, matched string, err error) {
	if oldString == newString {
		return "", "", fmt.Errorf("no changes to apply: old_string and new_string are identical")
	}
	if oldString == "" {
		return "", "", fmt.Errorf("old_string cannot be empty when editing an existing file; use write_file for intentional full-file replacement")
	}

	// Normalize CRLF for matching, then restore original line endings on write path.
	ending := detectLineEnding(content)
	normContent := normalizeLineEndings(content)
	normOld := normalizeLineEndings(oldString)
	normNew := normalizeLineEndings(newString)

	notFound := true
	for _, replacer := range editReplacers {
		for _, search := range replacer(normContent, normOld) {
			if search == "" {
				continue
			}
			if isDisproportionateMatch(search, normOld) {
				return "", "", fmt.Errorf("refusing replacement because the matched span is much larger than old_string; re-read the file and provide a more precise old_string")
			}
			idx := strings.Index(normContent, search)
			if idx < 0 {
				continue
			}
			notFound = false
			if replaceAll {
				count := strings.Count(normContent, search)
				if count == 0 {
					continue
				}
				next = strings.ReplaceAll(normContent, search, normNew)
				return convertToLineEnding(next, ending), search, nil
			}
			last := strings.LastIndex(normContent, search)
			if idx != last {
				// Ambiguous under this replacer; try next candidate / replacer.
				continue
			}
			next = normContent[:idx] + normNew + normContent[idx+len(search):]
			return convertToLineEnding(next, ending), search, nil
		}
	}

	if notFound {
		return "", "", fmt.Errorf("could not find old_string in the file; it must match exactly (or closely after fuzzy normalization), including indentation")
	}
	return "", "", fmt.Errorf("found multiple matches for old_string; provide more surrounding context to make the match unique, or set replace_all to true")
}

type editReplacer func(content, find string) []string

var editReplacers = []editReplacer{
	simpleReplacer,
	lineTrimmedReplacer,
	whitespaceNormalizedReplacer,
	indentationFlexibleReplacer,
	escapeNormalizedReplacer,
	trimmedBoundaryReplacer,
	contextAwareReplacer,
	multiOccurrenceReplacer,
}

func simpleReplacer(_content, find string) []string {
	return []string{find}
}

func lineTrimmedReplacer(content, find string) []string {
	originalLines := strings.Split(content, "\n")
	searchLines := strings.Split(find, "\n")
	if len(searchLines) > 0 && searchLines[len(searchLines)-1] == "" {
		searchLines = searchLines[:len(searchLines)-1]
	}
	if len(searchLines) == 0 {
		return nil
	}
	var out []string
	for i := 0; i <= len(originalLines)-len(searchLines); i++ {
		matches := true
		for j := 0; j < len(searchLines); j++ {
			if strings.TrimSpace(originalLines[i+j]) != strings.TrimSpace(searchLines[j]) {
				matches = false
				break
			}
		}
		if matches {
			out = append(out, joinLines(originalLines[i:i+len(searchLines)]))
		}
	}
	return out
}

func whitespaceNormalizedReplacer(content, find string) []string {
	normalizeWS := func(text string) string {
		return strings.Join(strings.Fields(text), " ")
	}
	normalizedFind := normalizeWS(find)
	var out []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if normalizeWS(line) == normalizedFind {
			out = append(out, line)
			continue
		}
		if strings.Contains(normalizeWS(line), normalizedFind) {
			words := strings.Fields(find)
			if len(words) == 0 {
				continue
			}
			parts := make([]string, len(words))
			for i, w := range words {
				parts[i] = regexp.QuoteMeta(w)
			}
			re, err := regexp.Compile(strings.Join(parts, `\s+`))
			if err != nil {
				continue
			}
			if m := re.FindString(line); m != "" {
				out = append(out, m)
			}
		}
	}
	findLines := strings.Split(find, "\n")
	if len(findLines) > 1 {
		for i := 0; i <= len(lines)-len(findLines); i++ {
			block := joinLines(lines[i : i+len(findLines)])
			if normalizeWS(block) == normalizedFind {
				out = append(out, block)
			}
		}
	}
	return uniqueStrings(out)
}

func indentationFlexibleReplacer(content, find string) []string {
	removeIndent := func(text string) string {
		lines := strings.Split(text, "\n")
		minIndent := -1
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := leadingWSLen(line)
			if minIndent < 0 || indent < minIndent {
				minIndent = indent
			}
		}
		if minIndent <= 0 {
			return text
		}
		var b strings.Builder
		for i, line := range lines {
			if i > 0 {
				b.WriteByte('\n')
			}
			if strings.TrimSpace(line) == "" {
				b.WriteString(line)
				continue
			}
			if len(line) >= minIndent {
				b.WriteString(line[minIndent:])
			} else {
				b.WriteString(line)
			}
		}
		return b.String()
	}
	normalizedFind := removeIndent(find)
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")
	var out []string
	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		block := joinLines(contentLines[i : i+len(findLines)])
		if removeIndent(block) == normalizedFind {
			out = append(out, block)
		}
	}
	return out
}

func escapeNormalizedReplacer(content, find string) []string {
	unescapedFind := unescapeCommon(find)
	var out []string
	if strings.Contains(content, unescapedFind) {
		out = append(out, unescapedFind)
	}
	lines := strings.Split(content, "\n")
	findLines := strings.Split(unescapedFind, "\n")
	for i := 0; i <= len(lines)-len(findLines); i++ {
		block := joinLines(lines[i : i+len(findLines)])
		if unescapeCommon(block) == unescapedFind {
			out = append(out, block)
		}
	}
	return uniqueStrings(out)
}

func trimmedBoundaryReplacer(content, find string) []string {
	trimmedFind := strings.TrimSpace(find)
	if trimmedFind == find {
		return nil
	}
	var out []string
	if strings.Contains(content, trimmedFind) {
		out = append(out, trimmedFind)
	}
	lines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")
	for i := 0; i <= len(lines)-len(findLines); i++ {
		block := joinLines(lines[i : i+len(findLines)])
		if strings.TrimSpace(block) == trimmedFind {
			out = append(out, block)
		}
	}
	return uniqueStrings(out)
}

func contextAwareReplacer(content, find string) []string {
	findLines := strings.Split(find, "\n")
	if len(findLines) > 0 && findLines[len(findLines)-1] == "" {
		findLines = findLines[:len(findLines)-1]
	}
	if len(findLines) < 3 {
		return nil
	}
	contentLines := strings.Split(content, "\n")
	firstLine := strings.TrimSpace(findLines[0])
	lastLine := strings.TrimSpace(findLines[len(findLines)-1])
	var out []string
	for i := 0; i < len(contentLines); i++ {
		if strings.TrimSpace(contentLines[i]) != firstLine {
			continue
		}
		for j := i + 2; j < len(contentLines); j++ {
			if strings.TrimSpace(contentLines[j]) != lastLine {
				continue
			}
			blockLines := contentLines[i : j+1]
			if len(blockLines) != len(findLines) {
				break
			}
			matching, total := 0, 0
			for k := 1; k < len(blockLines)-1; k++ {
				bl := strings.TrimSpace(blockLines[k])
				fl := strings.TrimSpace(findLines[k])
				if bl == "" && fl == "" {
					continue
				}
				total++
				if bl == fl {
					matching++
				}
			}
			if total == 0 || float64(matching)/float64(total) >= 0.5 {
				out = append(out, joinLines(blockLines))
			}
			break
		}
	}
	return out
}

func multiOccurrenceReplacer(content, find string) []string {
	var out []string
	start := 0
	for {
		idx := strings.Index(content[start:], find)
		if idx < 0 {
			break
		}
		out = append(out, find)
		start += idx + len(find)
		if len(find) == 0 {
			break
		}
	}
	return out
}

func isDisproportionateMatch(search, oldString string) bool {
	oldLines := strings.Count(oldString, "\n") + 1
	searchLines := strings.Count(search, "\n") + 1
	if searchLines >= maxInt(oldLines+3, oldLines*2) {
		return true
	}
	if oldLines == 1 {
		return false
	}
	oldTrim := len(strings.TrimSpace(oldString))
	searchTrim := len(strings.TrimSpace(search))
	return searchTrim > maxInt(oldTrim+500, oldTrim*4)
}

func normalizeLineEndings(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func detectLineEnding(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func convertToLineEnding(text, ending string) string {
	if ending == "\n" {
		return text
	}
	return strings.ReplaceAll(text, "\n", ending)
}

func unescapeCommon(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case 'n':
			b.WriteByte('\n')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case '\'', '"', '`', '\\', '$':
			b.WriteByte(s[i+1])
			i++
		case '\n':
			b.WriteByte('\n')
			i++
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func leadingWSLen(line string) int {
	n := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			n++
			continue
		}
		break
	}
	return n
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func uniqueStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// nearbyContext returns a small excerpt around the first line that loosely
// resembles needle, for error messages.
func nearbyContext(content, needle string, radius int) string {
	if radius <= 0 {
		radius = 2
	}
	lines := strings.Split(content, "\n")
	needleLines := strings.Split(strings.TrimSpace(needle), "\n")
	anchor := ""
	if len(needleLines) > 0 {
		anchor = strings.TrimSpace(needleLines[0])
	}
	if anchor == "" {
		return ""
	}
	idx := -1
	for i, line := range lines {
		if strings.Contains(line, anchor) || strings.TrimSpace(line) == anchor {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ""
	}
	start := idx - radius
	if start < 0 {
		start = 0
	}
	end := idx + radius
	if end >= len(lines) {
		end = len(lines) - 1
	}
	var b strings.Builder
	b.WriteString("nearby context:\n")
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%d: %s\n", i+1, lines[i])
	}
	return strings.TrimRight(b.String(), "\n")
}
