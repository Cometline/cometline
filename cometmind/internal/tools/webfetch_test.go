package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchSpecIsValidJSON(t *testing.T) {
	spec := WebFetch{}.Spec()
	if spec.Name != "web_fetch" {
		t.Fatalf("Name = %q, want web_fetch", spec.Name)
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v", err)
	}
}

func TestWebFetchExtractsHTMLText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>x</title><style>.a{}</style></head>` +
			`<body><h1>Hello</h1><script>alert(1)</script><p>World text</p></body></html>`))
	}))
	defer srv.Close()

	// Loopback is normally blocked by the SSRF guard; httptest binds to
	// 127.0.0.1, so this test exercises the guard rejecting it.
	res, err := WebFetch{}.Execute(context.Background(), mustJSON(t, map[string]any{"url": srv.URL}))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK {
		t.Fatalf("expected loopback to be blocked, got OK with output: %s", res.Output)
	}
	if !strings.Contains(res.Output, "private or local") && !strings.Contains(res.Output, "local address") {
		t.Errorf("expected SSRF-guard message, got: %s", res.Output)
	}
}

func TestWebFetchHTMLToText(t *testing.T) {
	got := htmlToText(`<html><head><style>.a{color:red}</style></head>` +
		`<body><h1>Title</h1><script>bad()</script><p>Para one</p><p>Para two</p></body></html>`)
	if strings.Contains(got, "color:red") || strings.Contains(got, "bad()") {
		t.Errorf("script/style leaked into text: %q", got)
	}
	if !strings.Contains(got, "Title") || !strings.Contains(got, "Para one") {
		t.Errorf("expected visible text, got: %q", got)
	}
}

func TestWebFetchRejectsNonHTTP(t *testing.T) {
	res, err := WebFetch{}.Execute(context.Background(), mustJSON(t, map[string]any{"url": "ftp://example.com"}))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK {
		t.Fatalf("expected non-http to be rejected")
	}
	if !strings.Contains(res.Output, "http(s)") {
		t.Errorf("expected scheme error, got: %s", res.Output)
	}
}

func TestWebFetchAcceptsBareStringURLInput(t *testing.T) {
	res, err := WebFetch{}.Execute(context.Background(), json.RawMessage(`"ftp://example.com"`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK {
		t.Fatalf("expected non-http to be rejected")
	}
	if !strings.Contains(res.Output, "http(s)") {
		t.Errorf("expected scheme error, got: %s", res.Output)
	}
}

func TestParseWebFetchInputPreservesMaxCharsForObjectInput(t *testing.T) {
	in, err := parseWebFetchInput(mustJSON(t, map[string]any{"url": "https://example.com", "max_chars": 123}))
	if err != nil {
		t.Fatalf("parseWebFetchInput error: %v", err)
	}
	if in.URL == nil || *in.URL != "https://example.com" || in.MaxChars != 123 {
		t.Fatalf("input = %+v, want URL and max_chars preserved", in)
	}
}

func TestParseWebFetchInputAcceptsStringifiedObject(t *testing.T) {
	in, err := parseWebFetchInput(json.RawMessage(`"{\"url\":\"https://example.com\",\"max_chars\":321}"`))
	if err != nil {
		t.Fatalf("parseWebFetchInput error: %v", err)
	}
	if in.URL == nil || *in.URL != "https://example.com" || in.MaxChars != 321 {
		t.Fatalf("input = %+v, want URL and max_chars preserved", in)
	}
}

func TestParseWebFetchInputAcceptsBareStringURL(t *testing.T) {
	in, err := parseWebFetchInput(json.RawMessage(`"https://example.com"`))
	if err != nil {
		t.Fatalf("parseWebFetchInput error: %v", err)
	}
	if in.URL == nil || *in.URL != "https://example.com" || in.MaxChars != 0 {
		t.Fatalf("input = %+v, want URL and max_chars preserved", in)
	}
}

func TestWebFetchRejectsEmptyURL(t *testing.T) {
	res, _ := WebFetch{}.Execute(context.Background(), mustJSON(t, map[string]any{"url": "  "}))
	if res.OK {
		t.Fatalf("expected empty url to be rejected")
	}
}

func TestWebFetchBlocksLocalhostHostname(t *testing.T) {
	res, _ := WebFetch{}.Execute(context.Background(), mustJSON(t, map[string]any{"url": "http://localhost:7700/"}))
	if res.OK {
		t.Fatalf("expected localhost to be blocked")
	}
}

func TestWebFetchInvalidJSONReturnsError(t *testing.T) {
	_, err := WebFetch{}.Execute(context.Background(), json.RawMessage(`{not json`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON input")
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
