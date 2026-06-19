package memory

import (
	"strconv"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestBuildRetrievalQuery_EmptyHistory(t *testing.T) {
	got := BuildRetrievalQuery(RetrievalQueryInput{
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hello"}},
		}},
	})
	if got != "Current message:\nhello" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildRetrievalQuery_WithRecentContext(t *testing.T) {
	msgs := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "old question"}}},
		{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "old answer"}}},
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "繼續"}}},
	}
	got := BuildRetrievalQuery(RetrievalQueryInput{Messages: msgs})
	if !strings.Contains(got, "user: old question") {
		t.Fatalf("missing recent user: %q", got)
	}
	if !strings.Contains(got, "assistant: old answer") {
		t.Fatalf("missing recent assistant: %q", got)
	}
	if !strings.Contains(got, "Current message:\n繼續") {
		t.Fatalf("missing current message: %q", got)
	}
}

func TestBuildRetrievalQuery_LimitsRecentTurns(t *testing.T) {
	var msgs []cometsdk.Message
	for i := 0; i < 5; i++ {
		msgs = append(msgs,
			cometsdk.Message{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "u" + strconv.Itoa(i)}}},
			cometsdk.Message{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "a" + strconv.Itoa(i)}}},
		)
	}
	msgs = append(msgs, cometsdk.Message{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "current"}},
	})

	got := BuildRetrievalQuery(RetrievalQueryInput{Messages: msgs})
	if strings.Contains(got, "u0") || strings.Contains(got, "a0") {
		t.Fatalf("expected oldest turns trimmed, got %q", got)
	}
	if !strings.Contains(got, "u3") || !strings.Contains(got, "a3") {
		t.Fatalf("expected recent turns kept, got %q", got)
	}
}

func TestBuildRetrievalQuery_SkipsToolResult(t *testing.T) {
	msgs := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "run tool"}}},
		{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{
			cometsdk.ToolCallBlock{ID: "tc1", Name: "read_file", Input: []byte(`{}`)},
		}},
		{Role: cometsdk.RoleToolResult, Content: []cometsdk.Block{
			cometsdk.ToolResultBlock{ToolCallID: "tc1", Content: "file contents"},
		}},
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "thanks"}}},
	}
	got := BuildRetrievalQuery(RetrievalQueryInput{Messages: msgs})
	if strings.Contains(got, "file contents") {
		t.Fatalf("tool result should be skipped: %q", got)
	}
	if !strings.Contains(got, "user: run tool") {
		t.Fatalf("expected user message in context: %q", got)
	}
}

func TestBuildRetrievalQuery_TruncatesLongSnippet(t *testing.T) {
	long := strings.Repeat("x", maxCharsPerSnippet+50)
	msgs := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: long}}},
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "ok"}}},
	}
	got := BuildRetrievalQuery(RetrievalQueryInput{Messages: msgs})
	if !strings.Contains(got, "…") {
		t.Fatalf("expected truncation marker: %q", got)
	}
}

func TestDecideRetrieval_SkipsLowValueTurn(t *testing.T) {
	decision := DecideRetrieval([]cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hihi!"}},
	}})
	if decision.Retrieve {
		t.Fatalf("Retrieve = true, want false: %+v", decision)
	}
	if decision.Reason != "ack_or_reaction" {
		t.Fatalf("reason = %q, want ack_or_reaction", decision.Reason)
	}
	if decision.TextBytes != len("hihi!") {
		t.Fatalf("textBytes = %d, want %d", decision.TextBytes, len("hihi!"))
	}
}

func TestDecideRetrieval_RetrievesShortQuestion(t *testing.T) {
	decision := DecideRetrieval([]cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Tom?"}},
	}})
	if !decision.Retrieve {
		t.Fatalf("Retrieve = false, want true: %+v", decision)
	}
	if decision.Reason != "question" {
		t.Fatalf("reason = %q, want question", decision.Reason)
	}
}

func TestDecideRetrieval_RetrievesImageTurn(t *testing.T) {
	decision := DecideRetrieval([]cometsdk.Message{{
		Role: cometsdk.RoleUser,
		Content: []cometsdk.Block{
			cometsdk.TextBlock{Text: "hihi"},
			cometsdk.ImageBlock{MediaType: "image/png", Data: "aGVsbG8="},
		},
	}})
	if !decision.Retrieve {
		t.Fatalf("Retrieve = false, want true: %+v", decision)
	}
	if decision.Reason != "media_turn" {
		t.Fatalf("reason = %q, want media_turn", decision.Reason)
	}
}

func TestDecideRetrieval_SkipsAckAndReaction(t *testing.T) {
	for _, text := range []string{"收到", "了解", "好喔", "嗯嗯", "哈哈", "lol", "XD", "👍"} {
		decision := DecideRetrieval([]cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: text}},
		}})
		if decision.Retrieve {
			t.Fatalf("%q Retrieve = true, want false: %+v", text, decision)
		}
	}
}

func TestDecideRetrieval_RetrievesPreferenceAndLanguageMemory(t *testing.T) {
	for _, text := range []string{"我偏好用 OpenAI embedding", "我偏好中文", "請之後用繁體中文回答"} {
		decision := DecideRetrieval([]cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: text}},
		}})
		if !decision.Retrieve {
			t.Fatalf("%q Retrieve = false, want true: %+v", text, decision)
		}
		if decision.Reason != "memory_keyword" {
			t.Fatalf("%q reason = %q, want memory_keyword", text, decision.Reason)
		}
	}
}

func TestDecideRetrieval_RetrievesTaskAndContextReference(t *testing.T) {
	for _, tc := range []struct {
		text   string
		reason string
	}{
		{text: "幫我看一下這段 code", reason: "task_request"},
		{text: "我想改後端 memory retrieval", reason: "task_request"},
		{text: "之前那件事", reason: "context_reference"},
		{text: "上次說的", reason: "context_reference"},
	} {
		decision := DecideRetrieval([]cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: tc.text}},
		}})
		if !decision.Retrieve {
			t.Fatalf("%q Retrieve = false, want true: %+v", tc.text, decision)
		}
		if decision.Reason != tc.reason {
			t.Fatalf("%q reason = %q, want %q", tc.text, decision.Reason, tc.reason)
		}
	}
}
