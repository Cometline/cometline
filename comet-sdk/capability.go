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

// CapabilitySet is an immutable set of capabilities. The zero value is empty.
type CapabilitySet struct {
	members map[Capability]struct{}
}

// NewCapabilitySet returns a set holding caps.
func NewCapabilitySet(caps ...Capability) CapabilitySet {
	if len(caps) == 0 {
		return CapabilitySet{}
	}
	members := make(map[Capability]struct{}, len(caps))
	for _, c := range caps {
		members[c] = struct{}{}
	}
	return CapabilitySet{members: members}
}

// Has reports whether c is in the set.
func (s CapabilitySet) Has(c Capability) bool {
	_, ok := s.members[c]
	return ok
}

// CapabilityOptions is the caller's knowledge about one model's optional
// features for a single request.
type CapabilityOptions struct {
	// Unsupported lists capabilities the caller already knows the model
	// rejects. Providers leave them out of the request up front.
	Unsupported CapabilitySet

	// OnUnsupported, if non-nil, is called synchronously from Provider.Stream
	// each time the endpoint rejects a capability and the provider retries
	// without it, so the caller can remember the rejection for later requests.
	OnUnsupported func(Capability)
}

// CapabilitySource resolves the capability options for one model scope.
type CapabilitySource interface {
	ResolveCapabilities(context.Context, CapabilityScope) CapabilityOptions
}

// CapabilityDisabled reports whether the caller marked c unsupported, through
// either Capabilities or the deprecated Compatibility policy. Provider
// implementations use it instead of reading those fields directly.
func (r *Request) CapabilityDisabled(c Capability) bool {
	if r.Capabilities.Unsupported.Has(c) {
		return true
	}
	return r.Compatibility != nil && r.Compatibility.Disabled(c)
}

// ReportUnsupportedCapability tells the caller that the endpoint rejected c.
// Provider implementations call it once per capability they fall back from.
func (r *Request) ReportUnsupportedCapability(c Capability) {
	if r.Capabilities.OnUnsupported != nil {
		r.Capabilities.OnUnsupported(c)
	}
	if r.Compatibility != nil {
		r.Compatibility.MarkUnsupported(c)
	}
}

// CapabilityPolicy records known unsupported capabilities for one model scope.
//
// Deprecated: providers no longer write back through the request. Use
// CapabilityOptions on Request.Capabilities; CapabilityPolicy is still
// honored for one release.
type CapabilityPolicy interface {
	Disabled(Capability) bool
	MarkUnsupported(Capability)
}

// CapabilityResolver returns a policy for one model scope.
//
// Deprecated: use CapabilitySource.
type CapabilityResolver interface {
	ResolveCapabilityPolicy(context.Context, CapabilityScope) CapabilityPolicy
}
