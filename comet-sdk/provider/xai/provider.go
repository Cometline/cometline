// Package xai implements xAI's OpenAI-compatible API using a Grok
// subscription OAuth session.
package xai

import (
	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/openai"
)

const defaultBaseURL = "https://api.x.ai"

// NewXAIProvider creates an xAI provider. OAuth credentials are read from the
// local subscription session; apiKey is retained only as a compatibility
// fallback for callers that explicitly provide one.
func NewXAIProvider(apiKey string, opts ...cometsdk.Option) cometsdk.Provider {
	return openai.NewOpenAICompatibleProvider(
		apiKey,
		"xai",
		BorrowToken,
		append([]cometsdk.Option{cometsdk.WithBaseURL(defaultBaseURL)}, opts...)...,
	)
}
