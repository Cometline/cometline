import { getContext, setContext } from 'svelte';
import type { AssistantStackContext } from './assistant-stack-props';

export const CHAT_TURN_CTX = Symbol('chat-turn');

export type ChatTurnContext = AssistantStackContext;

export function setChatTurnContext(ctx: ChatTurnContext) {
	setContext(CHAT_TURN_CTX, ctx);
}

export function getChatTurnContext(): ChatTurnContext {
	return getContext<ChatTurnContext>(CHAT_TURN_CTX);
}
