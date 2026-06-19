package memory

import "strings"

const defaultBaselinePreferenceLimit = 3

var validPreferenceCategories = map[string]struct{}{
	"language":     {},
	"tone":         {},
	"verbosity":    {},
	"workflow":     {},
	"model":        {},
	"tooling":      {},
	"coding_style": {},
	"other":        {},
}

var preferenceCategoryCaps = map[string]int{
	"language":     1,
	"verbosity":    1,
	"tone":         1,
	"model":        1,
	"tooling":      2,
	"workflow":     2,
	"coding_style": 2,
	"other":        2,
}

func normalizePreferenceCategory(kind, content, candidate string) string {
	if normalizeKind(kind) != "preference" {
		return ""
	}
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	candidate = strings.ReplaceAll(candidate, "-", "_")
	candidate = strings.ReplaceAll(candidate, " ", "_")
	if _, ok := validPreferenceCategories[candidate]; ok {
		return candidate
	}
	return inferPreferenceCategory(content)
}

func inferPreferenceCategory(content string) string {
	text := strings.ToLower(content)
	signals := []struct {
		category string
		words    []string
	}{
		{"language", []string{"language", "chinese", "english", "traditional chinese", "中文", "英文", "繁體中文", "回答語言"}},
		{"tone", []string{"tone", "polite", "casual", "friendly", "formal", "語氣", "口吻", "禮貌", "隨性", "正式"}},
		{"verbosity", []string{"concise", "detailed", "brief", "verbose", "short", "簡潔", "詳細", "長一點", "短一點"}},
		{"workflow", []string{"plan", "implement", "review", "ask me", "workflow", "先規劃", "先問", "工作流", "審查"}},
		{"model", []string{"model", "gpt", "claude", "haiku", "sonnet", "模型"}},
		{"tooling", []string{"tool", "shell", "terminal", "bash", "zsh", "工具", "終端"}},
		{"coding_style", []string{"code style", "testing", "test", "comments", "naming", "命名", "註解", "測試", "程式風格"}},
	}
	for _, signal := range signals {
		for _, word := range signal.words {
			if strings.Contains(text, word) {
				return signal.category
			}
		}
	}
	return "other"
}

func preferenceCategoryCap(category string) int {
	if cap, ok := preferenceCategoryCaps[category]; ok && cap > 0 {
		return cap
	}
	return preferenceCategoryCaps["other"]
}
