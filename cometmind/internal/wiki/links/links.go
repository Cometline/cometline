package links

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// ExtractTargets returns normalized wikilink targets from markdown content
// (page title / relative path without brackets; heading/alias stripped).
func ExtractTargets(content string) []string {
	matches := wikilinkPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		target := normalizeTarget(match[1])
		if target == "" {
			continue
		}
		key := strings.ToLower(target)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out
}

func normalizeTarget(inner string) string {
	trimmed := strings.TrimSpace(inner)
	if trimmed == "" {
		return ""
	}
	if pipe := strings.Index(trimmed, "|"); pipe >= 0 {
		trimmed = trimmed[:pipe]
	}
	if hash := strings.Index(trimmed, "#"); hash >= 0 {
		trimmed = trimmed[:hash]
	}
	trimmed = strings.TrimSpace(trimmed)
	ext := filepath.Ext(trimmed)
	if isWikiExt(ext) {
		trimmed = strings.TrimSuffix(trimmed, ext)
	}
	return strings.TrimSpace(trimmed)
}

func isWikiExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".html", ".htm":
		return true
	default:
		return false
	}
}

func stem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if isWikiExt(ext) {
		return strings.TrimSuffix(base, ext)
	}
	return base
}

func pathRank(path string) int {
	p := filepath.ToSlash(strings.ToLower(path))
	switch {
	case strings.HasPrefix(p, "entities/"):
		return 0
	case strings.HasPrefix(p, "concepts/"):
		return 1
	case strings.HasPrefix(p, "syntheses/"):
		return 2
	default:
		return 3
	}
}

// ResolveTarget maps a wikilink target onto a wiki-relative path from files.
func ResolveTarget(target string, files []string) string {
	raw := strings.TrimSpace(filepath.ToSlash(target))
	cleaned := normalizeTarget(target)
	if cleaned == "" || len(files) == 0 {
		return ""
	}

	candidates := []string{
		raw,
		cleaned,
		cleaned + ".md",
		cleaned + ".html",
		cleaned + ".htm",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		for _, file := range files {
			if strings.EqualFold(filepath.ToSlash(file), filepath.ToSlash(candidate)) {
				return filepath.ToSlash(file)
			}
		}
	}

	needle := strings.ToLower(filepath.Base(cleaned))
	var matches []string
	for _, file := range files {
		if strings.EqualFold(stem(file), needle) {
			matches = append(matches, filepath.ToSlash(file))
		}
	}
	if len(matches) == 0 {
		return ""
	}
	if len(matches) == 1 {
		return matches[0]
	}
	sort.Slice(matches, func(i, j int) bool {
		ri, rj := pathRank(matches[i]), pathRank(matches[j])
		if ri != rj {
			return ri < rj
		}
		return matches[i] < matches[j]
	})
	return matches[0]
}

// BuildBacklinkIndex scans wiki markdown (skipping raw/ and hidden entries) and
// returns map[targetPath][]sourcePath for resolved inbound links.
func BuildBacklinkIndex(ctx context.Context, root string) (map[string][]string, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]string{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return map[string][]string{}, nil
	}

	var files []string
	contents := make(map[string]string)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(name)
		if !isWikiExt(ext) {
			return nil
		}
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		// Only non-raw markdown bodies are scanned for [[wikilinks]]. HTML and
		// raw/** are kept in `files` so [[….html]] targets can resolve.
		if strings.EqualFold(ext, ".md") && !strings.HasPrefix(strings.ToLower(rel), "raw/") {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			contents[rel] = string(data)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	index := make(map[string][]string)
	for source, body := range contents {
		for _, target := range ExtractTargets(body) {
			resolved := ResolveTarget(target, files)
			if resolved == "" || resolved == source {
				continue
			}
			index[resolved] = append(index[resolved], source)
		}
	}

	for target, sources := range index {
		sort.Strings(sources)
		deduped := sources[:0]
		var prev string
		for _, source := range sources {
			if source == prev {
				continue
			}
			deduped = append(deduped, source)
			prev = source
		}
		index[target] = deduped
	}

	return index, nil
}

// BacklinksFor returns sorted inbound sources for path (wiki-relative).
func BacklinksFor(index map[string][]string, path string) []string {
	normalized := filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(path), "/"))
	if normalized == "" {
		return nil
	}
	if !isWikiExt(filepath.Ext(normalized)) {
		normalized += ".md"
	}
	sources := index[normalized]
	if sources == nil {
		// Case-insensitive fallback.
		for key, value := range index {
			if strings.EqualFold(key, normalized) {
				return append([]string(nil), value...)
			}
		}
		return nil
	}
	return append([]string(nil), sources...)
}
