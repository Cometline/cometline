package codex

import (
	"encoding/json"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/responsesproto"
)

// Wire aliases keep callers and tests on codex names while the protocol
// implementation lives in responsesproto (shared with the API-key Responses
// provider used by OpenCode Go).
type codexRequest = responsesproto.Request
type codexReasoning = responsesproto.Reasoning
type codexInput = responsesproto.InputItem
type codexContentPart = responsesproto.ContentPart
type codexTool = responsesproto.Tool

func toCodexRequest(req *cometsdk.Request, disableMaxOutputTokens, disableReasoningSummary, disableEncryptedReplay bool) ([]byte, error) {
	return responsesproto.BuildRequest(req, responsesproto.RequestOptions{
		ProviderKey:            providerID,
		DisableMaxOutputTokens:  disableMaxOutputTokens,
		DisableReasoningSummary: disableReasoningSummary,
		ReplayEncryptedState:    !disableEncryptedReplay,
	})
}

func addResponsesLiteReasoningContext(data []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok {
		reasoning = make(map[string]any)
	}
	reasoning["context"] = "all_turns"
	payload["reasoning"] = reasoning
	payload["parallel_tool_calls"] = false
	return json.Marshal(payload)
}
