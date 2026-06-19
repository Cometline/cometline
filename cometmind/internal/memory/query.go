package memory

import (
	"strings"
	"unicode"

	cometsdk "github.com/cometline/comet-sdk"
)

const (
	maxCharsPerSnippet     = 300
	maxRecentTurns         = 3
	maxRetrievalQueryChars = 6000
)

var lowValueRetrievalMessages = map[string]struct{}{
	"hi": {}, "hihi": {}, "hello": {}, "hey": {},
	"哈囉": {}, "嗨": {}, "你好": {},
	"ok": {}, "okay": {}, "k": {},
	"yes": {}, "yep": {}, "yeah": {}, "no": {}, "nope": {},
	"thanks": {}, "thank you": {}, "thx": {},
	"謝謝": {}, "感謝": {}, "好": {}, "好喔": {}, "好的": {}, "了解": {}, "收到": {}, "嗯": {}, "嗯嗯": {},
	"continue": {}, "go on": {}, "sure": {}, "nice": {}, "cool": {},
	"haha": {}, "lol": {}, "xd": {}, "哈哈": {}, "哈": {}, "www": {},
}

var questionSignals = []string{
	"?", "？", "what", "why", "how", "when", "where", "who",
	"怎麼", "為什麼", "如何", "哪個", "什麼", "嗎", "呢",
}

var taskSignals = []string{
	"help", "fix", "implement", "add", "change", "explain", "review", "debug", "search", "find",
	"幫我", "修", "改", "加", "實作", "解釋", "檢查", "看一下", "找",
}

var memorySignals = []string{
	"remember", "prefer", "preference", "default", "model", "setting", "project", "repo", "workspace",
	"記住", "偏好", "預設", "設定", "專案", "模型", "中文", "繁體中文", "回答語言",
}

var contextReferenceSignals = []string{
	"that", "previous", "before", "earlier", "last time",
	"那個", "之前", "上次", "剛剛", "前面",
}

var codeSignals = []string{"code", "api", "json", "yaml", "toml", "sql", "http", "func", "class", "const", "var", "src/", "internal/", ".go", ".ts", ".svelte"}

type RetrievalDecision struct {
	Retrieve  bool
	Reason    string
	TextBytes int
	Score     int
	Text      string
}

// RetrievalQueryInput carries session context for auto-retrieve query expansion.
type RetrievalQueryInput struct {
	Messages []cometsdk.Message
}

// BuildRetrievalQuery assembles an embedding query from recent dialogue and the
// current user message. No LLM call is involved.
func BuildRetrievalQuery(in RetrievalQueryInput) string {
	current := lastUserMessageText(in.Messages)
	if current == "" {
		return ""
	}

	var b strings.Builder
	recent := recentContextSnippets(in.Messages)
	if len(recent) > 0 {
		b.WriteString("Recent context:\n")
		for _, line := range recent {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Current message:\n")
	b.WriteString(current)

	out := b.String()
	if len(out) > maxRetrievalQueryChars {
		out = out[len(out)-maxRetrievalQueryChars:]
	}
	return out
}

// DecideRetrieval decides whether the latest user turn has enough signal to
// spend an embedding call on memory retrieval. It is intentionally local and
// deterministic: no LLM or embedding call is used to make this decision.
func DecideRetrieval(msgs []cometsdk.Message) RetrievalDecision {
	msg, ok := lastUserMessage(msgs)
	if !ok {
		return RetrievalDecision{Reason: "empty_user_message"}
	}
	text := messageTextFromSDK(msg)
	if userMessageHasNonTextBlock(msg) {
		return RetrievalDecision{Retrieve: true, Reason: "media_turn", TextBytes: len(text), Score: 3, Text: text}
	}
	normalized := normalizeLowValueText(text)
	if normalized == "" {
		return RetrievalDecision{Reason: "empty_user_message", TextBytes: len(text), Text: text}
	}
	if !hasMeaningfulRune(normalized) {
		return RetrievalDecision{Reason: "ack_or_reaction", TextBytes: len(text), Score: -3, Text: text}
	}
	if containsAny(normalized, questionSignals) {
		return RetrievalDecision{Retrieve: true, Reason: "question", TextBytes: len(text), Score: 3, Text: text}
	}

	score := 0
	reason := ""
	setSignal := func(nextReason string, points int) {
		score += points
		if reason == "" {
			reason = nextReason
		}
	}

	if containsAny(normalized, taskSignals) {
		setSignal("task_request", 3)
	}
	if containsAny(normalized, memorySignals) {
		setSignal("memory_keyword", 3)
	}
	if containsAny(normalized, contextReferenceSignals) {
		setSignal("context_reference", 3)
	}
	if containsAny(normalized, codeSignals) {
		setSignal("code_or_config", 2)
	}
	if isSubstantialText(normalized) {
		setSignal("long_substantive_text", 2)
	}
	if _, ok := lowValueRetrievalMessages[normalized]; ok {
		score -= 3
		if reason == "" {
			reason = "ack_or_reaction"
		}
	}
	if score >= 2 {
		return RetrievalDecision{Retrieve: true, Reason: reason, TextBytes: len(text), Score: score, Text: text}
	}
	if reason == "" {
		reason = "non_substantive_short_reply"
	}
	return RetrievalDecision{Reason: reason, TextBytes: len(text), Score: score, Text: text}
}

func lastUserMessage(msgs []cometsdk.Message) (cometsdk.Message, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == cometsdk.RoleUser {
			return msgs[i], true
		}
	}
	return cometsdk.Message{}, false
}

func userMessageHasNonTextBlock(m cometsdk.Message) bool {
	for _, bl := range m.Content {
		if _, ok := bl.(cometsdk.TextBlock); !ok {
			return true
		}
	}
	return false
}

func normalizeLowValueText(text string) string {
	text = strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
	text = strings.TrimRightFunc(text, func(r rune) bool {
		switch r {
		case '.', '!', '。', '！':
			return true
		default:
			return unicode.IsSpace(r)
		}
	})
	return strings.TrimSpace(text)
}

func containsAny(text string, signals []string) bool {
	for _, signal := range signals {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func hasMeaningfulRune(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func isSubstantialText(text string) bool {
	englishTokens := 0
	for _, token := range strings.Fields(text) {
		if containsASCIIAlnum(token) {
			englishTokens++
		}
	}
	return englishTokens >= 5 || meaningfulCJKRunes(text) >= 8
}

func containsASCIIAlnum(text string) bool {
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return true
		}
	}
	return false
}

func meaningfulCJKRunes(text string) int {
	count := 0
	for _, r := range text {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo) {
			count++
		}
	}
	return count
}

func recentContextSnippets(msgs []cometsdk.Message) []string {
	if len(msgs) <= 1 {
		return nil
	}
	// Exclude the final user message (current turn).
	prior := msgs[:len(msgs)-1]

	var snippets []string
	turns := 0
	for i := len(prior) - 1; i >= 0 && turns < maxRecentTurns; i-- {
		m := prior[i]
		switch m.Role {
		case cometsdk.RoleUser:
			text := truncateSnippet(messageTextFromSDK(m))
			if text == "" {
				continue
			}
			snippets = append([]string{"user: " + text}, snippets...)
			turns++
		case cometsdk.RoleAssistant:
			text := truncateSnippet(assistantTextFromSDK(m))
			if text == "" {
				continue
			}
			snippets = append([]string{"assistant: " + text}, snippets...)
		default:
			continue
		}
	}
	return snippets
}

func lastUserMessageText(msgs []cometsdk.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != cometsdk.RoleUser {
			continue
		}
		return messageTextFromSDK(msgs[i])
	}
	return ""
}

func messageTextFromSDK(m cometsdk.Message) string {
	var b strings.Builder
	for _, bl := range m.Content {
		if tb, ok := bl.(cometsdk.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func assistantTextFromSDK(m cometsdk.Message) string {
	var b strings.Builder
	for _, bl := range m.Content {
		if tb, ok := bl.(cometsdk.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func truncateSnippet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxCharsPerSnippet {
		return s
	}
	return s[:maxCharsPerSnippet] + "…"
}
