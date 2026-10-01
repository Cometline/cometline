package cometsdk

import "context"

// Capability identifies an optional model feature that may require a fallback.
type Capability string

const (
	CapabilityReasoningSummary         Capability = "reasoning_summary"
	CapabilityEncryptedReasoningReplay Capability = "encrypted_reasoning_replay"
	CapabilityMaxOutputTokens          Capability = "max_output_tokens"
	CapabilityToolInputStream          Capability = "tool_input_streaming"
)

// CapabilityScope identifies the configured model endpoint a policy applies to.
type CapabilityScope struct {
	ProviderID string
	Endpoint   string
	ModelID    string
}

// CapabilityPolicy records known unsupported capabilities for one model scope.
type CapabilityPolicy interface {
	Disabled(Capability) bool
	MarkUnsupported(Capability)
}

// CapabilityResolver returns a policy for one model scope.
type CapabilityResolver interface {
	ResolveCapabilityPolicy(context.Context, CapabilityScope) CapabilityPolicy
}
