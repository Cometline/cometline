package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	invalidToolArgumentsMarker = "invalid tool arguments"
	toolInputPreviewLimit      = 240
)

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

func isJSONSchemaError(err error) bool {
	if err == nil {
		return false
	}
	var syntax *json.SyntaxError
	var unmarshalType *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &unmarshalType) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "invalid character") ||
		strings.Contains(lower, "cannot unmarshal") ||
		strings.Contains(lower, "unexpected end of json")
}

// InvalidToolInputResult turns malformed model arguments into a recoverable
// tool error so the agent loop can send them back instead of aborting.
func InvalidToolInputResult(name string, input json.RawMessage, err error) Result {
	return Result{OK: false, InvalidInput: true, Output: formatInvalidToolInput(name, input, err)}
}

func formatInvalidToolInput(name string, input json.RawMessage, err error) string {
	detail := "arguments must be a complete JSON object matching the tool schema"
	if err != nil {
		detail = err.Error()
	}
	preview := previewToolInput(input)
	if preview == "" {
		preview = "<empty>"
	}
	return fmt.Sprintf(
		"%s for %s: %s. Retry this tool once with a smaller, complete JSON object. Raw input: %s",
		invalidToolArgumentsMarker, name, detail, preview,
	)
}

func previewToolInput(input json.RawMessage) string {
	s := strings.TrimSpace(string(input))
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= toolInputPreviewLimit {
		return s
	}
	runes := []rune(s)
	return string(runes[:toolInputPreviewLimit]) + "…"
}

// IsInvalidToolInput reports schema/JSON argument failures, including wrapped
// Execute errors and the recoverable Result produced by the registry.
// Tool output is never scanned: a successful read_file/grep whose content
// mentions the marker must not look like a schema failure.
func IsInvalidToolInput(res Result, err error) bool {
	if res.InvalidInput {
		return true
	}
	return isJSONSchemaError(err)
}

// IsCompleteJSONObject reports whether input is a finished JSON object.
// Truncated payloads and JSON strings are not complete objects.
func IsCompleteJSONObject(input json.RawMessage) bool {
	s := strings.TrimSpace(string(input))
	if s == "" || s[0] != '{' {
		return false
	}
	var obj map[string]json.RawMessage
	return json.Unmarshal([]byte(s), &obj) == nil
}
