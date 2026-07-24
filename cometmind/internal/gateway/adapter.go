package gateway

import "context"

// InboundMessage is a normalized message from an external chat platform.
type InboundMessage struct {
	Platform          string
	PlatformMessageID string
	GuildID           string
	ParentChannelID   string
	UserID            string
	ChannelID         string
	ThreadID          string
	Text              string
	Images            []InboundImage
	Mentioned         bool
}

// InboundImage is a normalized image attachment from an external chat platform.
type InboundImage struct {
	MediaType string
	Data      string
}

// TypingIndicator can show a platform-specific "typing" state while a turn runs.
type TypingIndicator interface {
	KeepTyping(ctx context.Context, channelID string) (stop func())
}

// OutboundMessage is a reply destined for an external chat platform.
type OutboundMessage struct {
	Platform  string
	UserID    string
	ChannelID string
	ThreadID  string
	Text      string
	Images    []OutboundImage
}

// OutboundImage is a local image file to attach to a platform reply.
type OutboundImage struct {
	Path      string
	Filename  string
	MediaType string
	Alt       string
}

// PlatformAdapter connects CometMind to one messaging surface.
type PlatformAdapter interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Deliver(ctx context.Context, msg OutboundMessage) error
	SetInboundHandler(fn func(context.Context, InboundMessage))
}
