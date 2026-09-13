package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestIsJSONSchemaError(t *testing.T) {
	if err := json.Unmarshal([]byte(`{"path":"/foo`), &map[string]any{}); !isJSONSchemaError(err) {
		t.Fatalf("truncated JSON should be a schema error: %v", err)
	}
	if err := json.Unmarshal([]byte(`"please list files"`), &struct {
		Path string `json:"path"`
	}{}); !isJSONSchemaError(err) {
		t.Fatalf("string payload should be a schema error: %v", err)
	}
	if isJSONSchemaError(errors.New("path is required")) {
		t.Fatal("validation errors must not look like schema failures")
	}
	if isJSONSchemaError(nil) {
		t.Fatal("nil error is not a schema failure")
	}
}

func TestInvalidToolInputResultIncludesPreviewAndRetryHint(t *testing.T) {
	res := InvalidToolInputResult("read_file", json.RawMessage(`{"path":"/foo`), errors.New("unexpected end of JSON input"))
	if res.OK {
		t.Fatal("expected failure result")
	}
	if !res.InvalidInput || !IsInvalidToolInput(res, nil) {
		t.Fatalf("result = %+v, want invalid tool arguments", res)
	}
	for _, want := range []string{"read_file", "unexpected end of JSON input", `{"path":"/foo`, "Retry this tool"} {
		if !strings.Contains(res.Output, want) {
			t.Fatalf("missing %q in:\n%s", want, res.Output)
		}
	}
}

func TestIsInvalidToolInputIgnoresOutputSubstring(t *testing.T) {
	res := Result{OK: true, Output: "1: invalid tool arguments for write_file: unexpected end of JSON input"}
	if IsInvalidToolInput(res, nil) {
		t.Fatalf("successful output containing the marker must not look like a schema failure: %+v", res)
	}
	failed := Result{OK: false, Output: "invalid tool arguments for write_file: unexpected end of JSON input"}
	if IsInvalidToolInput(failed, nil) {
		t.Fatalf("unmarked failure output must not look like a schema failure: %+v", failed)
	}
}

func TestIsCompleteJSONObject(t *testing.T) {
	if !IsCompleteJSONObject(json.RawMessage(`{"path":"hello.txt"}`)) {
		t.Fatal("valid object should be complete")
	}
	if IsCompleteJSONObject(json.RawMessage(`{"path":"/foo`)) {
		t.Fatal("truncated object should not be complete")
	}
	if IsCompleteJSONObject(json.RawMessage(`"please list files"`)) {
		t.Fatal("JSON string should not count as a complete object")
	}
	if IsCompleteJSONObject(json.RawMessage(`{"path":"hello.txt","content":"`)) {
		t.Fatal("truncated write_file payload should not be complete")
	}
}
