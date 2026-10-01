import type { WebContext } from '$lib/actions/start-chat';
import type { ChatItem } from '$lib/types';

const MAX_CONTEXT_TITLE_CHARS = 500;
const responseOrdinals = new WeakMap<readonly ChatItem[], Map<string, number>>();

export function normalizeAssistantSelection(text: string): string {
	return text.replace(/\s+/g, ' ').trim();
}

export function assistantResponseSource(
	sessionId: string,
	itemId: string,
	items: readonly ChatItem[]
): string | null {
	let ordinals = responseOrdinals.get(items);
	if (!ordinals) {
		ordinals = new Map();
		let ordinal = 0;
		for (const item of items) {
			if (item.type !== 'assistant') continue;
			ordinal += 1;
			ordinals.set(item.id, ordinal);
		}
		responseOrdinals.set(items, ordinals);
	}
	const ordinal = ordinals.get(itemId) ?? 0;
	return ordinal > 0 ? `assistant-response://${sessionId}/${ordinal}` : null;
}

export function buildAssistantResponseContext(opts: {
	sessionId: string;
	itemId: string;
	items: readonly ChatItem[];
	selectedText: string;
}): WebContext | null {
	const content = opts.selectedText.trim();
	const source = assistantResponseSource(opts.sessionId, opts.itemId, opts.items);
	if (!content || !source) return null;
	const normalized = normalizeAssistantSelection(content);
	return {
		kind: 'message',
		title: Array.from(normalized).slice(0, MAX_CONTEXT_TITLE_CHARS).join(''),
		source,
		content: Array.from(content).slice(0, 50000).join('')
	};
}
