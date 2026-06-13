package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
)

func TestNewOpenAIProviderUsesConfiguredBaseURL(t *testing.T) {
	var gotPath string
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()

	t.Setenv("COMETMIND_API_KEY", "dummy-key")

	p, err := New(&config.Config{
		Provider: config.ProviderOpenAI,
		Model:    "test-model",
		BaseURL:  srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := &cometsdk.Request{
		Model: "test-model",
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hello"}},
		}},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range ch {
	}

	if gotPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want %q", gotPath, "/v1/chat/completions")
	}
	if gotAuth != "Bearer dummy-key" {
		t.Fatalf("authorization header = %q, want %q", gotAuth, "Bearer dummy-key")
	}
}
