package memory

import (
	"fmt"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestRecentMessagesKeepsLastNMessages(t *testing.T) {
	msgs := make([]cometsdk.Message, 10)
	for i := range msgs {
		msgs[i] = cometsdk.Message{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: fmt.Sprintf("message-%d", i)}},
		}
	}

	got := recentMessages(msgs, 8)
	if len(got) != 8 {
		t.Fatalf("len = %d, want 8", len(got))
	}
	if messageText(got[0]) != "message-2" || messageText(got[7]) != "message-9" {
		t.Fatalf("kept wrong window: first=%q last=%q", messageText(got[0]), messageText(got[7]))
	}
}

func TestShouldSkipExtractionForAcknowledgementOnlyTurn(t *testing.T) {
	msgs := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Please remember I prefer compact answers."}}},
		{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Noted."}}},
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "thanks"}}},
	}

	if !shouldSkipExtraction(msgs) {
		t.Fatal("expected acknowledgement-only turn to skip extraction")
	}
}

func TestShouldNotSkipExtractionForSubstantiveTurn(t *testing.T) {
	msgs := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Remember that this project uses 8 messages for memory extraction."}}},
	}

	if shouldSkipExtraction(msgs) {
		t.Fatal("expected substantive turn to allow extraction")
	}
}
