import type { ChatItem } from '$lib/types';
import { anyReasoningPending, hasReasoning } from '$lib/features/chat/reasoning';
import type { StreamCtx } from '$lib/stores/chat-stream-types';

export type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

export function reconcileStreamCtx(cached: ChatItem[], ctx: StreamCtx) {
	if (ctx.assistant.current) {
		const synced = cached.find(
			(item): item is AssistantItem =>
				item.type === 'assistant' && item.id === ctx.assistant.current!.id
		);
		if (synced) {
			ctx.assistant.current = synced;
			return;
		}
		ctx.assistant.current = null;
	}
	const last = cached.at(-1);
	if (last?.type === 'assistant' && (last.pending === true || anyReasoningPending(last))) {
		ctx.assistant.current = last;
		return;
	}
	for (let i = cached.length - 1; i >= 0; i--) {
		const item = cached[i];
		if (item.type === 'assistant') {
			ctx.assistant.current = item;
			return;
		}
	}
}

/** True when the latest user turn left something the user can read. */
export function turnHasVisibleContent(
	sessionItems: ChatItem[],
	assistant: AssistantItem | null
): boolean {
	let lastUser = -1;
	for (let i = sessionItems.length - 1; i >= 0; i--) {
		if (sessionItems[i].type === 'user') {
			lastUser = i;
			break;
		}
	}
	const tail = lastUser >= 0 ? sessionItems.slice(lastUser + 1) : sessionItems;
	for (const item of tail) {
		if (item.type === 'error' && item.text.trim()) return true;
		if (
			item.type === 'assistant' &&
			(item.text.trim() || hasReasoning(item) || (item.images?.length ?? 0) > 0)
		)
			return true;
		if (item.type === 'tool' || item.type === 'subagent' || item.type === 'memory') return true;
	}
	if (assistant && (assistant.text.trim() || hasReasoning(assistant))) return true;
	return false;
}
