package memory

import (
	"strings"
	"unicode"

	cometsdk "github.com/cometline/comet-sdk"
)

var explicitRememberPrefixes = []string{
	"記住",
	"記得",
	"请记住",
	"請記住",
	"remember that ",
	"remember ",
	"don't forget ",
	"dont forget ",
	"do not forget ",
}

// tryExplicitRemember detects direct "remember this" user instructions and
// converts them into a memory proposal without calling the extraction LLM.
func tryExplicitRemember(msgs []cometsdk.Message) (proposedMemory, bool) {
	lastUser := strings.TrimSpace(lastUserMessageText(msgs))
	if lastUser == "" {
		return proposedMemory{}, false
	}
	lower := strings.ToLower(lastUser)
	for _, prefix := range explicitRememberPrefixes {
		idx := indexFold(lastUser, lower, prefix)
		if idx < 0 {
			continue
		}
		content := strings.TrimSpace(lastUser[idx+len(prefix):])
		content = trimEdgePunctuation(content)
		if len([]rune(content)) < 2 {
			continue
		}
		return proposedMemory{
			Content:    content,
			Kind:       "preference",
			Confidence: 0.9,
			ShouldSave: true,
		}, true
	}
	return proposedMemory{}, false
}

func indexFold(original, lower, prefix string) int {
	if strings.HasPrefix(lower, prefix) {
		return 0
	}
	needle := strings.ToLower(prefix)
	for i := 1; i < len(lower); i++ {
		if lower[i] != needle[0] {
			continue
		}
		if strings.HasPrefix(lower[i:], needle) {
			return i
		}
	}
	return -1
}

func trimEdgePunctuation(text string) string {
	return strings.TrimFunc(strings.TrimSpace(text), func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
}
