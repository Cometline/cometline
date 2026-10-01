package agent

import (
	"context"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

// TurnStore is the narrow persistence seam the agent loop drives. It is the
// subset of session.Service the Runner actually needs, declared here on the
// consumer side so the loop can be unit-tested with an in-memory fake instead
// of a live SQLite database. *session.Service satisfies it.
type TurnStore interface {
	BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error)
	SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage, providerID, modelID string) error
	AppendAssistantStep(ctx context.Context, sessionID, text string, reasoningBlocks []cometsdk.Block, toolCalls []cometsdk.ToolCallBlock, injectedMemories []session.InjectedMemory) (session.Message, map[string]string, error)
	UpdateToolCallResult(ctx context.Context, toolCallID, result string, durMs int64, exit *int64) error
	AppendToolResultMessage(ctx context.Context, sessionID, toolCallID, output string, isErr bool) (session.Message, error)
}

type providerStateStore interface {
	SaveAssistantProviderState(ctx context.Context, messageID string, states []cometsdk.ProviderState) error
}

type MemoryStore interface {
	Enabled() bool
	RetrieveForTurn(ctx context.Context, sessionID, query string, tokenAllowance int) (memory.PromptMemories, error)
	ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]memory.Change, error)
}

type sessionLoader interface {
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
}
