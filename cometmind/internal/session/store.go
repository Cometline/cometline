package session

import (
	"context"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
)

// WorkspaceStore is the narrow seam for workspace persistence.
// Modules that only need workspace CRUD should depend on this interface
// rather than the full *Service.
type WorkspaceStore interface {
	EnsureWorkspace(ctx context.Context, absRoot string) (Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error)
	LookupWorkspaceByPath(ctx context.Context, absRoot string) (Workspace, error)
	ListWorkspaces(ctx context.Context) ([]Workspace, error)
	WorkspacePath(ctx context.Context, workspaceID string) (string, error)
}

// SessionStore is the narrow seam for session persistence used by
// HTTP handlers, gateways, and retention. It is deliberately smaller
// than *Service so consumers can be unit-tested without SQLite.
type SessionStore interface {
	NewSession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	ListSessions(ctx context.Context, workspaceID string) ([]Session, error)
	ListAllSessions(ctx context.Context) ([]Session, error)
	ListChildSessions(ctx context.Context, parentSessionID string) ([]Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	ClearSessionTranscript(ctx context.Context, sessionID string) error
	ForkSession(ctx context.Context, sessionID, absPath string) (Session, error)
	ChangeSessionWorkspace(ctx context.Context, sessionID, absPath string) (Session, error)
	UpdateSessionModel(ctx context.Context, sessionID, modelID, providerID string) (Session, error)
	UpdateSessionPinned(ctx context.Context, sessionID string, pinned bool) (Session, error)
	UpdateSessionTitle(ctx context.Context, sessionID, title string) (Session, error)
	SetTitleIfEmpty(ctx context.Context, sessionID, title string) error
	UpdateDelegationState(ctx context.Context, sessionID string, status DelegationStatus, summary, pendingQuestion string) error

	// Gateway session binding.
	UpsertGatewaySession(ctx context.Context, platform, userID, channelID, threadID, sessionID, workspaceID string) (db.GatewaySession, error)
	LookupGatewaySession(ctx context.Context, platform, userID, channelID, threadID string) (db.GatewaySession, error)

	// Message and workspace helpers used by the gateway.
	AppendUserMessageContent(ctx context.Context, sessionID string, blocks []ContentBlock, displayText string, contexts []MessageContextRef) (Message, error)
	WorkspacePath(ctx context.Context, workspaceID string) (string, error)
	ListWorkspaces(ctx context.Context) ([]Workspace, error)
	EnsureWorkspace(ctx context.Context, absRoot string) (Workspace, error)
}

// TranscriptReader is the narrow seam the memory extractor needs to
// build the message history used for memory extraction. It is declared
// in the consumer package so memory can be tested without a real session store.
type TranscriptReader interface {
	BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error)
}

// MessageAppender is the narrow seam used by the gateway to append
// user content and set titles without depending on the full *Service.
type MessageAppender interface {
	AppendUserMessageContent(ctx context.Context, sessionID string, blocks []ContentBlock, displayText string, contexts []MessageContextRef) (Message, error)
	SetTitleIfEmpty(ctx context.Context, sessionID, title string) error
}

// AssistantMediaAppender persists assistant-presented images for chat UIs.
type AssistantMediaAppender interface {
	AppendAssistantMedia(ctx context.Context, sessionID string, images []ContentBlock) (Message, error)
}

// AssistantMediaMetaAppender persists assistant media with catalog metadata.
type AssistantMediaMetaAppender interface {
	AssistantMediaAppender
	AppendAssistantMediaWithMeta(ctx context.Context, sessionID string, items []ContentBlock, meta MediaMeta) (Message, error)
}

// ReadyImage is a session-local still that can be reused as a video first frame.
type ReadyImage struct {
	ID        string
	MediaType string
	Data      []byte
}

// ReadyMediaReader loads a ready image that belongs to the current session.
type ReadyMediaReader interface {
	ReadySessionImage(ctx context.Context, sessionID, mediaID string) (ReadyImage, error)
}

// ChildSessionReader is the narrow seam used by subagent tools to create,
// update, and read child sessions without depending on the full *Service.
type ChildSessionReader interface {
	GetSession(ctx context.Context, sessionID string) (Session, error)
	NewChildSession(ctx context.Context, parent Session, purpose, subagentKind string) (Session, error)
	UpdateSessionModel(ctx context.Context, sessionID, modelID, providerID string) (Session, error)
	AppendUserMessage(ctx context.Context, sessionID, text string) (Message, error)
	UpdateDelegationState(ctx context.Context, sessionID string, status DelegationStatus, summary, pendingQuestion string) error
	UpdateACPSessionID(ctx context.Context, sessionID, acpSessionID string) error
	CompactChildSession(ctx context.Context, childID string) error
	LastAssistantText(ctx context.Context, sessionID string) (string, error)
	ListToolCallsForSession(ctx context.Context, sessionID string) ([]db.ToolCall, error)
}

// MessageRowsReader is the narrow seam for reading raw message rows.
type MessageRowsReader interface {
	ListMessageRows(ctx context.Context, sessionID string) ([]db.Message, error)
}

// SDKMessagesAllReader is the narrow seam for reading all SDK messages
// without compaction filtering.
type SDKMessagesAllReader interface {
	BuildSDKMessagesAll(ctx context.Context, sessionID string) ([]cometsdk.Message, error)
}

// ToolCallsReader is the narrow seam for reading tool calls.
type ToolCallsReader interface {
	ListToolCallsForSession(ctx context.Context, sessionID string) ([]db.ToolCall, error)
}

// CompactorStore is the narrow seam for the compaction module.
type CompactorStore interface {
	MessageRowsReader
	TranscriptReader
	ToolCallsReader
	UpdateContextSummary(ctx context.Context, sessionID, summary, untilMessageID string) error
}

// Compile-time assertions that *Service satisfies the narrow seams.
var (
	_ WorkspaceStore         = (*Service)(nil)
	_ SessionStore           = (*Service)(nil)
	_ TranscriptReader       = (*Service)(nil)
	_ MessageAppender        = (*Service)(nil)
	_ AssistantMediaAppender = (*Service)(nil)
	_ ChildSessionReader     = (*Service)(nil)
	_ CompactorStore         = (*Service)(nil)
	_ MessageRowsReader      = (*Service)(nil)
	_ SDKMessagesAllReader   = (*Service)(nil)
	_ ToolCallsReader        = (*Service)(nil)
)
