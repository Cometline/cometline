package openai

import (
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

func TestHoldbackTagPrefix_RedactedThinkingPartial(t *testing.T) {
	s := "<redacted_thi"
	require.Equal(t, len(s), holdbackTagPrefix(s, openTags()))
}

func TestContentReasoningSplitter_PartialOpenTag(t *testing.T) {
	var s contentReasoningSplitter
	events := s.push("<redacted_thi")
	require.Empty(t, events)

	events = s.push("nking>warm</redacted_" + "thinking>Hi")
	var reasoning, texts []string
	for _, e := range events {
		switch ev := e.(type) {
		case cometsdk.ReasoningContentEvent:
			reasoning = append(reasoning, ev.Text)
		case cometsdk.TextDeltaEvent:
			texts = append(texts, ev.Text)
		}
	}
	require.Equal(t, []string{"warm"}, reasoning)
	require.Equal(t, []string{"Hi"}, texts)
}
