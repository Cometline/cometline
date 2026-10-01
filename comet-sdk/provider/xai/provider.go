// Package xai implements xAI's OpenAI-compatible API using a Grok
// subscription OAuth session.
package xai

import (
	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/provider/openai"
)

const defaultBaseURL = "https://api.x.ai"

// New creates an xAI provider. OAuth credentials are read from the local
// subscription session; apiKey is retained only as a compatibility fallback
// for callers that explicitly provide one.
func New(apiKey string, opts ...cometsdk.Option) cometsdk.Provider {
	return openai.NewCompatible(
		apiKey,
		"xai",
		BorrowToken,
		append([]cometsdk.Option{cometsdk.WithBaseURL(defaultBaseURL)}, opts...)...,
	)
}

// NewXAIProvider creates an xAI provider.
//
// Deprecated: use New.
func NewXAIProvider(apiKey string, opts ...cometsdk.Option) cometsdk.Provider {
	return New(apiKey, opts...)
}
