package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Embedder turns text into a dense vector.
type Embedder interface {
	Model() string
	Embed(ctx context.Context, texts ...string) ([][]float32, error)
}

type openAIEmbedder struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
}

func NewOpenAIEmbedder(baseURL, apiKey, model string) Embedder {
	if strings.TrimSpace(model) == "" {
		model = "text-embedding-3-small"
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &openAIEmbedder{
		client:  &http.Client{Timeout: 60 * time.Second},
		baseURL: base,
		apiKey:  apiKey,
		model:   model,
	}
}

func (e *openAIEmbedder) Model() string { return e.model }

func (e *openAIEmbedder) Embed(ctx context.Context, texts ...string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embeddings API: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

type ollamaEmbedder struct {
	client  *http.Client
	baseURL string
	model   string
}

func NewOllamaEmbedder(baseURL, model string) Embedder {
	if strings.TrimSpace(model) == "" {
		model = "nomic-embed-text"
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:11434"
	}
	return &ollamaEmbedder{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: base,
		model:   model,
	}
}

func (e *ollamaEmbedder) Model() string { return e.model }

func (e *ollamaEmbedder) Embed(ctx context.Context, texts ...string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		body, err := json.Marshal(map[string]string{
			"model":  e.model,
			"prompt": text,
		})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := e.client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("ollama embeddings: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
		}
		var parsed struct {
			Embedding []float32 `json:"embedding"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, err
		}
		out[i] = parsed.Embedding
	}
	return out, nil
}

func isOpenAICompatibleEmbeddingProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "openai-compatible", "opencode-go", "":
		return true
	default:
		return false
	}
}

func NewEmbedder(cfg EmbeddingSettings) (Embedder, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "ollama":
		return NewOllamaEmbedder(cfg.BaseURL, cfg.Model), nil
	case "openai", "openai-compatible", "opencode-go", "":
		return NewOpenAIEmbedder(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider %q (use openai, openai-compatible, ollama, or opencode-go)", cfg.Provider)
	}
}
