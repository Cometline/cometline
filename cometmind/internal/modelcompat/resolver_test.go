package modelcompat

import (
	"context"
	"database/sql"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func newTestResolver(t *testing.T) *Resolver {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return New(db.New(conn))
}

func TestResolverPersistsUnsupportedCapability(t *testing.T) {
	resolver := newTestResolver(t)
	scope := cometsdk.CapabilityScope{ProviderID: "codex", Endpoint: "default", ModelID: "gpt-test"}

	opts := resolver.ResolveCapabilities(context.Background(), scope)
	if opts.Unsupported.Has(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("reasoning summary unexpectedly unsupported")
	}
	opts.OnUnsupported(cometsdk.CapabilityReasoningSummary)

	reloaded := resolver.ResolveCapabilities(context.Background(), scope)
	if !reloaded.Unsupported.Has(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("reasoning summary was not restored from the negative cache")
	}
	other := resolver.ResolveCapabilities(context.Background(), cometsdk.CapabilityScope{ProviderID: "codex", Endpoint: "default", ModelID: "gpt-other"})
	if other.Unsupported.Has(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("negative cache leaked into another model scope")
	}
}

func TestResolverLegacyPolicyPersistsUnsupportedCapability(t *testing.T) {
	resolver := newTestResolver(t)
	scope := cometsdk.CapabilityScope{ProviderID: "codex", Endpoint: "default", ModelID: "gpt-test"}

	policy := resolver.ResolveCapabilityPolicy(context.Background(), scope)
	if policy.Disabled(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("reasoning summary unexpectedly disabled")
	}
	policy.MarkUnsupported(cometsdk.CapabilityReasoningSummary)
	if !policy.Disabled(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("marked capability should be disabled for the rest of the request")
	}

	reloaded := resolver.ResolveCapabilityPolicy(context.Background(), scope)
	if !reloaded.Disabled(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("reasoning summary was not restored from the negative cache")
	}
}

func TestNilResolverResolvesEmptyOptions(t *testing.T) {
	var resolver *Resolver
	opts := resolver.ResolveCapabilities(context.Background(), cometsdk.CapabilityScope{})
	if opts.Unsupported.Has(cometsdk.CapabilityReasoningSummary) || opts.OnUnsupported != nil {
		t.Fatalf("nil resolver options = %+v, want zero value", opts)
	}
}
