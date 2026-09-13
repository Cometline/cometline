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
	if !IsInvalidToolInput(res, nil) {
		t.Fatalf("result = %+v, want invalid tool arguments", res)
	}
	for _, want := range []string{"read_file", "unexpected end of JSON input", `{"path":"/foo`, "Retry this tool"} {
		if !strings.Contains(res.Output, want) {
			t.Fatalf("missing %q in:\n%s", want, res.Output)
		}
	}
}
