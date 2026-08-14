package llm

import cometsdk "github.com/cometline/comet-sdk"

func buildAssistantMessage(text, reasoning string, toolCalls []cometsdk.ToolCallBlock, providerState []cometsdk.ProviderState) cometsdk.Message {
	var blocks []cometsdk.Block
	if text != "" || len(toolCalls) > 0 {
		blocks = make([]cometsdk.Block, 0, len(toolCalls)+1)
	}
	if text != "" {
		blocks = append(blocks, cometsdk.TextBlock{Text: text})
	}
	for _, toolCall := range toolCalls {
		blocks = append(blocks, toolCall)
	}

	var reasoningBlocks []cometsdk.Block
	if reasoning != "" {
		reasoningBlocks = append(reasoningBlocks, cometsdk.ReasoningBlock{Text: reasoning})
	}

	return cometsdk.Message{
		Role:             cometsdk.RoleAssistant,
		Content:          blocks,
		ReasoningContent: reasoningBlocks,
		ProviderState:    providerState,
	}
}
