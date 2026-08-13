package codex

import (
	"encoding/json"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/responsesproto"
)

func toCodexRequest(req *cometsdk.Request, disableMaxOutputTokens, disableReasoningSummary, disableEncryptedReplay bool) ([]byte, error) {
	return responsesproto.BuildRequest(req, responsesproto.RequestOptions{
		ProviderKey:             providerID,
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
