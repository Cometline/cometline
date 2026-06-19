package tools

import "strings"

func requiredTrimmedString(value *string, field string) (string, Result, bool) {
	if value == nil {
		return "", Result{OK: false, Output: field + " is required"}, false
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return "", Result{OK: false, Output: field + " is required"}, false
	}
	return trimmed, Result{}, true
}

func requiredString(value *string, field string) (string, Result, bool) {
	if value == nil {
		return "", Result{OK: false, Output: field + " is required"}, false
	}
	return *value, Result{}, true
}
