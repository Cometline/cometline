package provider

import "testing"

func TestNormalizeOllamaNativeBase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", defaultOllamaNativeBase},
		{"http://127.0.0.1:11434", "http://127.0.0.1:11434"},
		{"http://127.0.0.1:11434/", "http://127.0.0.1:11434"},
		{"http://127.0.0.1:11434/v1", "http://127.0.0.1:11434"},
		{"http://127.0.0.1:11434/v1/", "http://127.0.0.1:11434"},
	}
	for _, tc := range cases {
		if got := NormalizeOllamaNativeBase(tc.in); got != tc.want {
			t.Fatalf("NormalizeOllamaNativeBase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := OllamaChatBaseURL("http://127.0.0.1:11434"); got != "http://127.0.0.1:11434/v1" {
		t.Fatalf("OllamaChatBaseURL = %q", got)
	}
	if !IsLoopbackOllamaURL("http://127.0.0.1:11434/v1") {
		t.Fatal("expected loopback")
	}
	if IsLoopbackOllamaURL("https://api.openai.com/v1") {
		t.Fatal("expected non-loopback")
	}
}
