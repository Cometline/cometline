package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
)

func TestThreadCreationParent(t *testing.T) {
	t.Parallel()

	parentID, parentType, ok := threadCreationParent(&discordgo.Channel{ID: "text-1", Type: discordgo.ChannelTypeGuildText})
	if !ok || parentID != "text-1" || parentType != discordgo.ChannelTypeGuildText {
		t.Fatalf("text channel parent = (%q, %d, %v), want (text-1, GuildText, true)", parentID, parentType, ok)
	}

	parentID, parentType, ok = threadCreationParent(&discordgo.Channel{
		ID:       "thread-1",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		ParentID: "forum-1",
	})
	if !ok || parentID != "forum-1" || parentType != 0 {
		t.Fatalf("thread parent = (%q, %d, %v), want (forum-1, 0, true)", parentID, parentType, ok)
	}
}

func TestChannelTypeHelpers(t *testing.T) {
	t.Parallel()

	if !isForumLikeChannelType(discordgo.ChannelTypeGuildForum) {
		t.Fatal("expected forum channel to be forum-like")
	}
	if !isForumLikeChannelType(discordgo.ChannelTypeGuildMedia) {
		t.Fatal("expected media channel to be forum-like")
	}
	if isForumLikeChannelType(discordgo.ChannelTypeGuildText) {
		t.Fatal("expected text channel not to be forum-like")
	}
	if !supportsPublicThreadCreation(discordgo.ChannelTypeGuildText) {
		t.Fatal("expected text channel to support public threads")
	}
	if supportsPublicThreadCreation(discordgo.ChannelTypeGuildForum) {
		t.Fatal("expected forum channel not to use text thread API")
	}
}

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
