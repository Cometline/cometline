package memory

import "testing"

func TestNewEmbedderProviders(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  bool
	}{
		{provider: "openai"},
		{provider: "openai-compatible"},
		{provider: "opencode-go"},
		{provider: "ollama"},
		{provider: "unknown", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			_, err := NewEmbedder(EmbeddingSettings{
				Provider: tt.provider,
				Model:    "test-model",
				BaseURL:  "http://127.0.0.1:8080/v1",
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewEmbedder(%q) err=%v wantErr=%v", tt.provider, err, tt.wantErr)
			}
		})
	}
}

func TestIsOpenAICompatibleEmbeddingProvider(t *testing.T) {
	for _, p := range []string{"openai", "openai-compatible", "opencode-go", ""} {
		if !isOpenAICompatibleEmbeddingProvider(p) {
			t.Fatalf("%q should use OpenAI-compatible embeddings API", p)
		}
	}
	if isOpenAICompatibleEmbeddingProvider("ollama") {
		t.Fatal("ollama should not use OpenAI-compatible embeddings API")
	}
}
