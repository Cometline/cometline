package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWebSearchSpecIsValidJSON(t *testing.T) {
	spec := (WebSearch{}).Spec()
	if spec.Name != "web_search" {
		t.Fatalf("Name = %q, want web_search", spec.Name)
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v", err)
	}
}

func TestWebSearchUsesBrowserBridge(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.Header.Get("X-Cometline-Browser-Token") != "secret" {
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("bad request")), Header: make(http.Header)}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"query":"go testing","backend":"electron-chromium-google","results":[{"title":"Go","url":"https://go.dev","snippet":"The Go programming language"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	result, err := (WebSearch{Endpoint: "http://browser.test/search", Token: "secret", Client: client}).Execute(
		context.Background(),
		json.RawMessage(`{"query":"go testing","limit":3}`),
	)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.OK || !strings.Contains(result.Output, "https://go.dev") {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Output, "electron-chromium") {
		t.Fatalf("backend missing from result: %s", result.Output)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestParseDuckDuckGoResults(t *testing.T) {
	results := parseDuckDuckGoResults(`<div class="result"><a class="result__a" href="https://example.com/a">Example A</a><a class="result__snippet">Snippet A</a></div>`+`<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fb">Example B</a><div class="result__snippet">Snippet B</div></div>`, 5)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(results), results)
	}
	if results[0].URL != "https://example.com/a" || results[0].Snippet != "Snippet A" {
		t.Fatalf("first result = %+v", results[0])
	}
	if results[1].URL != "https://example.com/b" {
		t.Fatalf("redirect URL was not normalized: %+v", results[1])
	}
}

func TestWebSearchRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{`{"query":""}`, `{"query":"ok","limit":0}` /* limit 0 defaults */} {
		result, err := (WebSearch{}).Execute(context.Background(), json.RawMessage(input))
		if err != nil {
			t.Fatalf("input %s: Execute error: %v", input, err)
		}
		if input == `{"query":""}` && result.OK {
			t.Fatalf("empty query unexpectedly succeeded")
		}
	}
}
