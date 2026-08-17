package generation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestXAIClientGenerateImageDownloadsURL(t *testing.T) {
	var generations int
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/images/generations", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok-1" {
			t.Fatalf("auth = %q", got)
		}
		generations++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != DefaultImageModel || body["prompt"] != "a cat" {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": srv.URL + "/file.png"}},
		})
	})
	mux.HandleFunc("/file.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	})

	client := NewXAIClient(srv.Client())
	client.BaseURL = srv.URL
	client.Borrow = func(context.Context, *http.Client) (string, error) { return "tok-1", nil }

	got, err := client.GenerateImage(context.Background(), ImageRequest{
		Prompt: "a cat",
		Model:  DefaultImageModel,
	})
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(got.Data) != "png-bytes" || got.MediaType != "image/png" {
		t.Fatalf("result = %#v", got)
	}
	if generations != 1 {
		t.Fatalf("generations = %d", generations)
	}
}

func TestXAIClientGenerateVideoPollsThenDownloads(t *testing.T) {
	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/generations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"request_id": "req-9",
				"status":     "pending",
			})
		case r.URL.Path == "/videos/req-9":
			polls++
			if polls < 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{"request_id": "req-9", "status": "processing"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"request_id": "req-9",
				"status":     "done",
				"url":        "http://" + r.Host + "/clip.mp4",
			})
		case r.URL.Path == "/clip.mp4":
			_, _ = io.WriteString(w, "mp4-bytes")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	now := time.Unix(0, 0)
	client := NewXAIClient(srv.Client())
	client.BaseURL = srv.URL
	client.Borrow = func(context.Context, *http.Client) (string, error) { return "tok-1", nil }
	client.Now = func() time.Time { return now }
	client.Sleep = func(time.Duration) { now = now.Add(time.Second) }

	got, err := client.GenerateVideo(context.Background(), VideoRequest{Prompt: "lift off"})
	if err != nil {
		t.Fatalf("GenerateVideo: %v", err)
	}
	if string(got.Data) != "mp4-bytes" || got.MediaType != "video/mp4" {
		t.Fatalf("result = %#v", got)
	}
	if polls < 2 {
		t.Fatalf("polls = %d", polls)
	}
}

func TestXAIClientRetriesTransientTimeouts(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			time.Sleep(250 * time.Millisecond)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": "cG5n"}},
		})
	}))
	t.Cleanup(srv.Close)

	client := NewXAIClient(&http.Client{Timeout: 80 * time.Millisecond})
	client.BaseURL = srv.URL
	client.Borrow = func(context.Context, *http.Client) (string, error) { return "tok-1", nil }
	client.Sleep = func(time.Duration) {}

	got, err := client.GenerateImage(context.Background(), ImageRequest{Prompt: "retry me"})
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(got.Data) != "png" {
		t.Fatalf("data = %q", got.Data)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
}

func TestXAIClientRefusesUntrustedDownloadURL(t *testing.T) {
	client := NewXAIClient(&http.Client{})
	client.BaseURL = xaiBaseURL
	client.Borrow = func(context.Context, *http.Client) (string, error) { return "tok-secret", nil }
	_, err := client.download(context.Background(), "https://evil.example/steal")
	if err == nil || !strings.Contains(err.Error(), "untrusted") {
		t.Fatalf("error = %v", err)
	}
	_, err = client.download(context.Background(), "http://127.0.0.1:9/secret")
	if err == nil || !strings.Contains(err.Error(), "untrusted") {
		t.Fatalf("loopback error = %v", err)
	}
}

func TestXAIClientGenerateImageReportsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"oauth gated"}`)
	}))
	t.Cleanup(srv.Close)
	client := NewXAIClient(srv.Client())
	client.BaseURL = srv.URL
	client.Borrow = func(context.Context, *http.Client) (string, error) { return "tok-1", nil }

	_, err := client.GenerateImage(context.Background(), ImageRequest{Prompt: "nope"})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
}
