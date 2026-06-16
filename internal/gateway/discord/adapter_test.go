package discord

import (
	"testing"

	"github.com/cometline/cometmind/internal/gateway"
)

func TestDiscordRoutingIDs(t *testing.T) {
	t.Parallel()

	parent, thread := discordRoutingIDs("chan-1", "")
	if parent != "chan-1" || thread != "" {
		t.Fatalf("parent channel routing = (%q, %q), want (chan-1, \"\")", parent, thread)
	}

	parent, thread = discordRoutingIDs("thread-1", "chan-1")
	if parent != "chan-1" || thread != "thread-1" {
		t.Fatalf("thread routing = (%q, %q), want (chan-1, thread-1)", parent, thread)
	}
}

func TestDeliveryChannelID(t *testing.T) {
	t.Parallel()

	if got := deliveryChannelID(gateway.OutboundMessage{ChannelID: "chan-1"}); got != "chan-1" {
		t.Fatalf("deliveryChannelID() = %q, want chan-1", got)
	}
	if got := deliveryChannelID(gateway.OutboundMessage{ChannelID: "chan-1", ThreadID: "thread-1"}); got != "thread-1" {
		t.Fatalf("deliveryChannelID() = %q, want thread-1", got)
	}
}
