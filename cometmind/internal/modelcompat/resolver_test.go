package modelcompat

import (
	"context"
	"database/sql"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestResolverPersistsUnsupportedCapability(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	resolver := New(db.New(conn))
	scope := cometsdk.CapabilityScope{ProviderID: "codex", Endpoint: "default", ModelID: "gpt-test"}
	policy := resolver.ResolveCapabilityPolicy(context.Background(), scope)
	if policy.Disabled(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("reasoning summary unexpectedly disabled")
	}
	policy.MarkUnsupported(cometsdk.CapabilityReasoningSummary)

	reloaded := resolver.ResolveCapabilityPolicy(context.Background(), scope)
	if !reloaded.Disabled(cometsdk.CapabilityReasoningSummary) {
		t.Fatal("reasoning summary was not restored from the negative cache")
	}
}
