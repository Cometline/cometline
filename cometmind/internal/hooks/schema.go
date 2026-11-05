package hooks

import "encoding/json"

const (
	EventPreToolUse  = "PreToolUse"
	EventPostToolUse = "PostToolUse"

	maxChainDepth     = 10
	defaultTimeoutSec = 30
	maxTimeoutSec     = 86400
)

// ToolEnv is serialized to hook stdin.
type ToolEnv struct {
	HookEvent     string          `json:"hook_event"`
	SessionID     string          `json:"session_id"`
	WorkspacePath string          `json:"workspace_path"`
	TurnID        string          `json:"turn_id"` // Same as SessionID today; see hookToolEnv in internal/agent.
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	ModelID       string          `json:"model_id"`
	ProviderID    string          `json:"provider_id"`
	Platform      string          `json:"platform"`
	JobID         string          `json:"job_id,omitempty"`
	ToolOutput    string          `json:"tool_output,omitempty"`
	ToolOK        bool            `json:"tool_ok,omitempty"`
}

// Outcome is parsed from hook stdout JSON.
type Outcome struct {
	Block           bool            `json:"block"`
	BlockReason     string          `json:"block_reason"`
	AppendSystem    string          `json:"append_system"`
	ModifyToolInput json.RawMessage `json:"modify_tool_input"`
}

// PreResult aggregates PreToolUse hook effects.
type PreResult struct {
	Blocked      bool
	BlockReason  string
	Input        json.RawMessage
	AppendSystem []string
}

// PostResult aggregates PostToolUse hook effects.
type PostResult struct {
	AppendSystem []string
}

// CatalogEntry describes one configured hook for settings / API listing.
type CatalogEntry struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Command string `json:"command"`
	Source  string `json:"source"`
}
