package llm

import (
	"encoding/json"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"b\":2}\n```", `{"b":2}`},
		{"Here is the result:\n```\n{\"c\":3}\n```\nThanks", `{"c":3}`},
		{"prefix {\"d\":4} suffix", `{"d":4}`},
	}
	for _, tt := range tests {
		raw, err := extractJSON(tt.in)
		if err != nil {
			t.Fatalf("extractJSON(%q): %v", tt.in, err)
		}
		if string(raw) != tt.want {
			t.Fatalf("got %q want %q", raw, tt.want)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
	}
}

func TestExtractJSONInvalid(t *testing.T) {
	if _, err := extractJSON("not json"); err == nil {
		t.Fatal("expected error")
	}
}
