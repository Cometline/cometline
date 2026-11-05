package hooks

import (
	"regexp"
	"strings"
)

func matcherMatches(matcher, toolName string) bool {
	matcher = strings.TrimSpace(matcher)
	toolName = strings.TrimSpace(toolName)
	if matcher == "" || matcher == "*" {
		return true
	}
	re, err := regexp.Compile(matcher)
	if err != nil {
		return matcher == toolName
	}
	return re.MatchString(toolName)
}
