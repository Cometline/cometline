package cometsdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/provider/openai"
)

// chatCompletionsSSE is a canned OpenAI Chat Completions stream, so the
// example runs without network access or an API key.
const chatCompletionsSSE = `data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{"content":", world!"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}

data: [DONE]

`

// Construct a provider and consume its raw event stream. Point
// cometsdk.WithBaseURL at a real endpoint (or drop it to use the default)
// in production code.
func Example() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, chatCompletionsSSE)
	}))
	defer srv.Close()

	p := openai.New("sk-example",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithLogger(nil),
	)

	events, err := p.Stream(context.Background(), &cometsdk.Request{
		Model:     "gpt-4o",
		MaxTokens: 256,
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Say hello."}},
		}},
	})
	if err != nil {
		fmt.Println("stream:", err)
		return
	}

	for ev := range events {
		switch e := ev.(type) {
		case cometsdk.TextDeltaEvent:
			fmt.Print(e.Text)
		case cometsdk.StepFinishEvent:
			fmt.Printf("\nfinish=%s in=%d out=%d\n", e.FinishReason, e.Usage.InputTokens, e.Usage.OutputTokens)
		case cometsdk.ErrorEvent:
			fmt.Println("error:", e.Err)
		}
	}
	// Output:
	// Hello, world!
	// finish=stop in=10 out=5
}

func ExampleNormalizeBaseURL() {
	fmt.Println(cometsdk.NormalizeBaseURL("https://api.example.com/v1/"))
	// Output: https://api.example.com/v1
}

func ExampleNormalizeFinishReason() {
	for _, raw := range []string{"end_turn", "tool_calls", "length"} {
		fmt.Println(raw, "->", cometsdk.NormalizeFinishReason(raw))
	}
	// Output:
	// end_turn -> stop
	// tool_calls -> tool_use
	// length -> max_tokens
}
