package agent

import (
	"context"
	"strings"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"go.uber.org/zap"
)

func assistantPlainText(m cometsdk.Message) string {
	var b strings.Builder
	for _, bl := range m.Content {
		if tb, ok := bl.(cometsdk.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return b.String()
}

func persistPartialStep(ctx context.Context, store TurnStore, sessionID, providerID, modelID string, result *llm.GenerateMessageResult, memories []session.InjectedMemory) {
	if result == nil {
		return
	}
	partialText := strings.TrimSpace(assistantPlainText(result.Message))
	if partialText == "" && len(result.Message.ReasoningContent) == 0 && len(result.Message.ProviderState) == 0 {
		return
	}
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer persistCancel()
	assistant, _, err := store.AppendAssistantStep(persistCtx, sessionID, partialText, result.Message.ReasoningContent, nil, memories)
	if err != nil {
		logging.L().Warn("agent.partial_persist_failed", zap.String("session", sessionID), zap.Error(err))
		return
	}
	if states, ok := store.(providerStateStore); ok {
		if err := states.SaveAssistantProviderState(persistCtx, assistant.ID, scopeProviderState(result.Message.ProviderState, providerID, modelID)); err != nil {
			logging.L().Warn("agent.partial_provider_state_persist_failed", zap.String("session", sessionID), zap.Error(err))
		}
	}
}

func scopeProviderState(states []cometsdk.ProviderState, providerID, modelID string) []cometsdk.ProviderState {
	if len(states) == 0 {
		return nil
	}
	result := append([]cometsdk.ProviderState(nil), states...)
	for i := range result {
		result[i].ProviderID = providerID
		if result[i].ModelID == "" {
			result[i].ModelID = modelID
		}
	}
	return result
}
