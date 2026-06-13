// Package cometsdk is a provider-agnostic Go LLM client library.
// It provides a single Provider interface that works the same way for
// Anthropic, OpenAI, and other LLM backends.
package cometsdk

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Provider is the single interface implemented by every LLM backend.
// Callers only ever interact with this interface — never with concrete types.
type Provider interface {
	// ID returns the provider identifier, e.g. "anthropic", "openai".
	ID() string

	// Stream sends a request to the LLM and returns a channel of Events.
	// The channel is closed when the LLM finishes or the context is cancelled.
	// Errors during streaming are sent as ErrorEvent before the channel closes.
	// Pre-stream errors (e.g. all retries exhausted) are returned as the error value.
	Stream(ctx context.Context, req *Request) (<-chan Event, error)
}

// ─── Request ──────────────────────────────────────────────────────────────────

// Request is the provider-agnostic input to Stream().
type Request struct {
	// Model is the model identifier as the provider expects it,
	// e.g. "claude-sonnet-4-5", "gpt-4o".
	Model string

	// Messages is the conversation history.
	Messages []Message

	// Tools is the list of tools the LLM may call.
	// Empty slice means no tool calling.
	Tools []Tool

	// System is the system prompt. Empty string means no system prompt.
	System string

	// MaxTokens caps the output. 0 means use the provider default.
	MaxTokens int

	// Temperature controls randomness. nil means use the provider default.
	Temperature *float64

	// Options holds provider-specific overrides.
	// Most callers leave this nil.
	Options map[string]any
}

// ─── Message & Blocks ─────────────────────────────────────────────────────────

// Message is a single turn in the conversation.
type Message struct {
	Role           Role      // RoleUser | RoleAssistant | RoleToolResult
	Content        []Block   // one or more content blocks
	ReasoningContent []Block // reasoning content (e.g. chain-of-thought); may be nil
}

// Role identifies the speaker of a message.
type Role string

const (
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolResult Role = "tool_result"
)

// Block is a sealed interface — only the concrete types below implement it.
type Block interface {
	isBlock()
}

// TextBlock is a plain text content block.
type TextBlock struct {
	Text string
}

func (TextBlock) isBlock() {}

// ReasoningBlock represents a reasoning / chain-of-thought content block.
// It is structurally identical to TextBlock but carries a distinct type
// so callers and providers can distinguish reasoning from final text.
type ReasoningBlock struct {
	Text string
}

func (ReasoningBlock) isBlock() {}

// ToolCallBlock represents a tool invocation emitted by the assistant.
type ToolCallBlock struct {
	ID    string          // provider-assigned call ID
	Name  string          // tool name
	Input json.RawMessage // JSON-encoded arguments
}

func (ToolCallBlock) isBlock() {}

// ToolResultBlock is the result of a tool call, sent back by the caller.
type ToolResultBlock struct {
	ToolCallID string // matches ToolCallBlock.ID
	Content    string // tool output (text)
	IsError    bool   // true if the tool failed
}

func (ToolResultBlock) isBlock() {}

// ─── Tool ─────────────────────────────────────────────────────────────────────

// Tool describes a function the LLM can call.
// Parameters must be a valid JSON Schema object.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema: {"type":"object","properties":{...}}
}

// ─── Events ───────────────────────────────────────────────────────────────────

// Event is a sealed interface. All event types below implement it.
type Event interface {
	isEvent()
}

// TextDeltaEvent carries a streamed text token from the LLM.
type TextDeltaEvent struct {
	Text string
}

func (TextDeltaEvent) isEvent() {}

// ToolCallStartEvent signals the LLM has started a tool call.
// Input may be partial at this point; wait for ToolCallDoneEvent for complete input.
type ToolCallStartEvent struct {
	ID   string
	Name string
}

func (ToolCallStartEvent) isEvent() {}

// ToolCallDeltaEvent carries a partial JSON fragment of the tool input.
type ToolCallDeltaEvent struct {
	ID    string
	Delta string
}

func (ToolCallDeltaEvent) isEvent() {}

// ToolCallDoneEvent signals the tool call input is complete.
type ToolCallDoneEvent struct {
	ID    string
	Name  string
	Input json.RawMessage // complete, valid JSON
}

func (ToolCallDoneEvent) isEvent() {}

// FinishReason is the normalized vocabulary every provider must emit.
// Providers translate their own stop reasons into one of these values so
// callers never have to know provider-specific spellings (e.g. Anthropic's
// "end_turn" or OpenAI's "length").
const (
	FinishStop      = "stop"       // model completed its turn normally
	FinishToolUse   = "tool_use"   // model wants one or more tools invoked
	FinishMaxTokens = "max_tokens" // output was truncated by a token limit
	FinishError     = "error"      // the step ended in an error
)

// NormalizeFinishReason maps a provider-specific stop reason onto the canonical
// FinishReason vocabulary. Unknown non-empty reasons fall back to FinishStop so
// the agent loop always terminates on a completed turn; empty stays empty.
func NormalizeFinishReason(raw string) string {
	switch raw {
	case "", FinishStop, FinishToolUse, FinishMaxTokens, FinishError:
		return raw
	// Anthropic stop_reason vocabulary.
	case "end_turn", "stop_sequence":
		return FinishStop
	// OpenAI finish_reason vocabulary.
	case "tool_calls":
		return FinishToolUse
	case "length":
		return FinishMaxTokens
	case "content_filter":
		return FinishStop
	default:
		return FinishStop
	}
}

// StepFinishEvent is emitted at the end of each LLM call (one "step").
type StepFinishEvent struct {
	// FinishReason is always one of the canonical Finish* constants.
	FinishReason string
	Usage        TokenUsage
}

func (StepFinishEvent) isEvent() {}

// ErrorEvent carries a streaming error. After an ErrorEvent the channel is closed.
type ErrorEvent struct {
	Err error
}

func (ErrorEvent) isEvent() {}

// DoneEvent is the final event. After this the channel is closed.
type DoneEvent struct{}

func (DoneEvent) isEvent() {}

// ReasoningStartEvent signals the start of a reasoning block.
type ReasoningStartEvent struct{}

func (ReasoningStartEvent) isEvent() {}

// ReasoningContentEvent carries a streamed reasoning token.
type ReasoningContentEvent struct {
	Text string
}

func (ReasoningContentEvent) isEvent() {}

// ─── TokenUsage ───────────────────────────────────────────────────────────────

// TokenUsage reports token consumption for a single LLM step.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int // prompt cache read tokens (Anthropic)
	CacheWrite   int // prompt cache write tokens (Anthropic)
}

// ─── Options ──────────────────────────────────────────────────────────────────

// AuthMode controls how the API key is sent in the request.
type AuthMode int

const (
	// AuthModeDefault uses the provider's native auth header.
	// Anthropic → X-API-Key, OpenAI → Authorization: Bearer.
	AuthModeDefault AuthMode = iota

	// AuthModeBearer forces Authorization: Bearer <key> regardless of provider.
	// Use this when pointing any provider at a unified API gateway.
	AuthModeBearer
)

// ProviderConfig holds configuration common to all provider implementations.
// It is populated by applying Option functions and is embedded by each provider.
type ProviderConfig struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	MaxRetries int
	AuthMode   AuthMode
	// Logger receives structured debug-level traces of SSE events and retries.
	// Defaults to slog.Default() if nil.
	Logger *slog.Logger
}

// DefaultProviderConfig returns sensible defaults shared by all providers.
// Each provider should set its own BaseURL before applying user options.
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		HTTPClient: &http.Client{},
		Timeout:    2 * time.Minute,
		MaxRetries: 4,
		Logger:     slog.Default(),
	}
}

// NormaliseBaseURL strips a trailing slash from a base URL so that appending
// paths like "/v1/chat/completions" never produces a double slash.
//
// Examples:
//
//	"https://api.example.com/v1/"  → "https://api.example.com/v1"
//	"https://api.example.com/v1"   → "https://api.example.com/v1"
func NormaliseBaseURL(u string) string {
	return strings.TrimRight(u, "/")
}

// Option is a functional option for provider configuration.
type Option func(*ProviderConfig)

// WithBaseURL overrides the provider's default API base URL.
func WithBaseURL(url string) Option {
	return func(c *ProviderConfig) {
		c.BaseURL = url
	}
}

// WithHTTPClient replaces the default http.Client.
// Useful for injecting custom transports, proxies, or test clients.
func WithHTTPClient(client *http.Client) Option {
	return func(c *ProviderConfig) {
		c.HTTPClient = client
	}
}

// WithTimeout sets the per-request timeout.
// Defaults to 2 minutes if not set.
func WithTimeout(d time.Duration) Option {
	return func(c *ProviderConfig) {
		c.Timeout = d
	}
}

// WithMaxRetries sets the maximum number of attempts (including the first).
// Defaults to 4. Set to 1 to disable retries.
func WithMaxRetries(n int) Option {
	return func(c *ProviderConfig) {
		c.MaxRetries = n
	}
}

// WithLogger sets a custom *slog.Logger for debug-level event tracing.
// Pass slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
// to see detailed SSE event and retry logs.
// Passing nil silences all SDK logging.
func WithLogger(l *slog.Logger) Option {
	return func(c *ProviderConfig) {
		c.Logger = l
	}
}

// WithBearerAuth forces the provider to send the API key as
// "Authorization: Bearer <key>" instead of its native auth header.
//
// Use this when pointing any provider at a unified API gateway that
// normalises authentication — e.g. a company endpoint that accepts
// Bearer tokens for both Anthropic and OpenAI requests.
func WithBearerAuth() Option {
	return func(c *ProviderConfig) {
		c.AuthMode = AuthModeBearer
	}
}
