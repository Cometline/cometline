import { getContext, setContext } from 'svelte';
import type { AssistantStackContext } from './assistant-stack-props';

export const CHAT_TURN_CTX = Symbol('chat-turn');

export type ChatTurnContext = AssistantStackContext;

type ChatTurnContextSource = ChatTurnContext | { readonly value: ChatTurnContext };

function resolveContext(source: ChatTurnContextSource | undefined): ChatTurnContext {
	if (!source) {
		throw new Error('getChatTurnContext() must be used inside AssistantStack');
	}
	return 'value' in source ? source.value : source;
}

export function setChatTurnContext(ctx: ChatTurnContext) {
	setContext(CHAT_TURN_CTX, ctx);
}

/** Register context during component init; reads the latest prop on each access. */
export function setReactiveChatTurnContext(getCtx: () => ChatTurnContext) {
	setContext(CHAT_TURN_CTX, {
		get value() {
			return getCtx();
		}
	} satisfies ChatTurnContextSource);
}

export function getChatTurnContext(): ChatTurnContext {
	return resolveContext(getContext<ChatTurnContextSource>(CHAT_TURN_CTX));
}
