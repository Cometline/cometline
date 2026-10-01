package cometsdk

import (
	"slices"
	"testing"
)

type legacyPolicy struct {
	disabled map[Capability]bool
	marked   []Capability
}

func (p *legacyPolicy) Disabled(c Capability) bool { return p.disabled[c] }
func (p *legacyPolicy) MarkUnsupported(c Capability) {
	p.marked = append(p.marked, c)
}

func TestCapabilitySet(t *testing.T) {
	t.Parallel()

	var empty CapabilitySet
	if empty.Has(CapabilityReasoningSummary) {
		t.Fatal("zero CapabilitySet should be empty")
	}
	set := NewCapabilitySet(CapabilityReasoningSummary, CapabilityMaxOutputTokens)
	if !set.Has(CapabilityReasoningSummary) || !set.Has(CapabilityMaxOutputTokens) {
		t.Fatal("set should contain its members")
	}
	if set.Has(CapabilityToolInputStream) {
		t.Fatal("set should not contain non-members")
	}
}

func TestRequestCapabilityDisabled(t *testing.T) {
	t.Parallel()

	req := &Request{
		Capabilities:  CapabilityOptions{Unsupported: NewCapabilitySet(CapabilityReasoningSummary)},
		Compatibility: &legacyPolicy{disabled: map[Capability]bool{CapabilityMaxOutputTokens: true}},
	}
	for c, want := range map[Capability]bool{
		CapabilityReasoningSummary: true,
		CapabilityMaxOutputTokens:  true,
		CapabilityToolInputStream:  false,
	} {
		if got := req.CapabilityDisabled(c); got != want {
			t.Errorf("CapabilityDisabled(%s) = %v, want %v", c, got, want)
		}
	}
	if (&Request{}).CapabilityDisabled(CapabilityReasoningSummary) {
		t.Error("zero Request should not disable capabilities")
	}
}

func TestRequestReportUnsupportedCapability(t *testing.T) {
	t.Parallel()

	var reported []Capability
	legacy := &legacyPolicy{}
	req := &Request{
		Capabilities:  CapabilityOptions{OnUnsupported: func(c Capability) { reported = append(reported, c) }},
		Compatibility: legacy,
	}
	req.ReportUnsupportedCapability(CapabilityEncryptedReasoningReplay)

	want := []Capability{CapabilityEncryptedReasoningReplay}
	if !slices.Equal(reported, want) {
		t.Errorf("OnUnsupported got %v, want %v", reported, want)
	}
	if !slices.Equal(legacy.marked, want) {
		t.Errorf("legacy MarkUnsupported got %v, want %v", legacy.marked, want)
	}

	(&Request{}).ReportUnsupportedCapability(CapabilityReasoningSummary)
}
