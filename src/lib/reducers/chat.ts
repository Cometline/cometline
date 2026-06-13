import type { ChatItem, StreamEvent } from '$lib/types';

export interface ChatState {
	items: ChatItem[];
	error: string;
	assistant: Extract<ChatItem, { type: 'assistant' }> | null;
	reasoning: { text: string; pending: boolean } | null;
	nextId: number;
}

export function initChatState(): ChatState {
	return { items: [], error: '', assistant: null, reasoning: null, nextId: 0 };
}

type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

function localID(prefix: string, nextId: number): { id: string; nextId: number } {
	return { id: `${prefix}-${Date.now()}-${nextId}`, nextId: nextId + 1 };
}

function cleanErrorMessage(message: string) {
	let text = message.trim();
	const jsonStart = text.indexOf('{');
	if (jsonStart >= 0) {
		try {
			const parsed = JSON.parse(text.slice(jsonStart));
			text = parsed?.error?.message || parsed?.message || text;
		} catch {
			// Keep the original message if the server body is not JSON.
		}
	}
	if (text.includes('OPENAI_API_KEY') || text.includes('COMETMIND_API_KEY')) {
		return 'API key is missing. Open Settings with Command+, and save your provider API key.';
	}
	return text.replace(/^\d+:\s*/, '') || 'The request failed.';
}

function removeEmptyAssistant(items: ChatItem[], assistant: AssistantItem | null): ChatItem[] {
	if (!assistant) return items;
	if (assistant.text.trim() || assistant.reasoning?.text.trim()) return items;
	return items.filter((item) => item.id !== assistant.id);
}

function attachReasoning(
	assistant: AssistantItem,
	reasoning: { text: string; pending: boolean }
): AssistantItem {
	if (!reasoning.text.trim() && !reasoning.pending) return assistant;
	const chunk = reasoning.text;
	if (assistant.reasoning?.text) {
		return {
			...assistant,
			reasoning: { text: assistant.reasoning.text + '\n\n' + chunk, pending: reasoning.pending }
		};
	}
	return { ...assistant, reasoning: { text: chunk, pending: reasoning.pending } };
}

function settleTurn(ctx: {
	assistant: AssistantItem | null;
	reasoning: { text: string; pending: boolean } | null;
}) {
	if (ctx.reasoning) ctx.reasoning.pending = false;
	if (ctx.assistant?.reasoning) ctx.assistant.reasoning.pending = false;
	if (ctx.assistant && ctx.reasoning) {
		if (ctx.assistant.reasoning) {
			ctx.assistant.reasoning.pending = ctx.reasoning.pending;
		} else {
			const next = attachReasoning(ctx.assistant, ctx.reasoning);
			ctx.assistant.text = next.text;
			ctx.assistant.reasoning = next.reasoning;
		}
		ctx.reasoning = null;
	}
	if (ctx.assistant) {
		ctx.assistant.pending = false;
		if (ctx.assistant.reasoning) ctx.assistant.reasoning.pending = false;
	}
}

function applyEvent(
	draft: ChatState,
	event: StreamEvent,
	ctx: {
		assistant: { current: AssistantItem | null };
		reasoning: { current: { text: string; pending: boolean } | null };
	}
) {
	const { assistant, reasoning } = ctx;
	const { items, nextId } = draft;

	function pushAssistant(next: AssistantItem) {
		items.push(next);
		assistant.current = next;
	}

	function ensureReasoningHost() {
		if (assistant.current) return assistant.current;
		const id = localID('assistant', draft.nextId++).id;
		const next: AssistantItem = {
			id,
			type: 'assistant',
			text: '',
			reasoning: { text: '', pending: true }
		};
		pushAssistant(next);
		return next;
	}

	function ensureAssistantForText() {
		if (assistant.current) return assistant.current;
		const id = localID('assistant', draft.nextId++).id;
		const next: AssistantItem = { id, type: 'assistant', text: '' };
		pushAssistant(next);
		return next;
	}

	function clearEmptyAssistant() {
		if (!assistant.current) return;
		draft.items = removeEmptyAssistant(draft.items, assistant.current);
		assistant.current = null;
	}

	function finishAssistantSegment() {
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		if (assistant.current && !assistant.current.text.trim() && !assistant.current.reasoning?.text.trim()) {
			clearEmptyAssistant();
		} else {
			assistant.current = null;
		}
		reasoning.current = null;
	}

	function ensureTurnReasoning() {
		if (!reasoning.current) reasoning.current = { text: '', pending: true };
		return reasoning.current;
	}

	function syncReasoningPreview() {
		if (!reasoning.current) return ensureReasoningHost();
		const host = ensureReasoningHost();
		host.reasoning = { text: reasoning.current.text, pending: reasoning.current.pending };
		return host;
	}

	if (event.type === 'reasoning_start') {
		if (reasoning.current?.text) {
			reasoning.current.text += '\n\n';
		} else {
			reasoning.current = { text: '', pending: true };
		}
		const host = syncReasoningPreview();
		host.pending = true;
		return;
	}

	if (event.type === 'reasoning_delta') {
		const turnReasoning = ensureTurnReasoning();
		turnReasoning.text += event.text;
		const host = syncReasoningPreview();
		host.pending = true;
		return;
	}

	if (event.type === 'text_delta') {
		const nextAssistant = ensureAssistantForText();
		if (nextAssistant.reasoning) {
			nextAssistant.reasoning.pending = false;
		}
		if (reasoning.current) reasoning.current.pending = false;
		reasoning.current = null;
		nextAssistant.text += event.delta;
		nextAssistant.pending = false;
		return;
	}

	if (event.type === 'tool_call') {
		finishAssistantSegment();
		const id = localID('tool', draft.nextId++).id;
		items.push({
			id,
			type: 'tool',
			toolId: event.id,
			toolName: event.tool,
			input: event.input,
			pending: true
		});
		return;
	}

	if (event.type === 'tool_result') {
		const tool = items.find((item) => item.type === 'tool' && item.toolId === event.id) as
			| Extract<ChatItem, { type: 'tool' }>
			| undefined;
		if (tool) {
			tool.output = event.output;
			tool.error = event.error;
			tool.pending = false;
		}
		return;
	}

	if (event.type === 'step_finish') {
		finishAssistantSegment();
		return;
	}

	if (event.type === 'error') {
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		clearEmptyAssistant();
		draft.error = cleanErrorMessage(event.message);
		const id = localID('error', draft.nextId++).id;
		items.push({ id, type: 'error', text: draft.error });
		return;
	}

	if (event.type === 'done') {
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		if (assistant.current && !assistant.current.text.trim()) {
			clearEmptyAssistant();
		}
	}
}

function cloneReasoning(r: { text: string; pending: boolean } | null): { text: string; pending: boolean } | null {
	return r ? { text: r.text, pending: r.pending } : null;
}

function cloneAssistant(a: AssistantItem | null): AssistantItem | null {
	if (!a) return null;
	return {
		...a,
		reasoning: a.reasoning ? { text: a.reasoning.text, pending: a.reasoning.pending } : undefined
	};
}

function cloneItem(item: ChatItem): ChatItem {
	if (item.type === 'assistant') {
		return {
			...item,
			reasoning: item.reasoning
				? { text: item.reasoning.text, pending: item.reasoning.pending }
				: undefined
		};
	}
	return { ...item };
}

function cloneChatState(state: ChatState): ChatState {
	const itemMap = new Map<ChatItem, ChatItem>();
	const items = state.items.map((item) => {
		const clone = cloneItem(item);
		itemMap.set(item, clone);
		return clone;
	});
	const assistant = state.assistant
		? ((itemMap.get(state.assistant) as AssistantItem | undefined) ??
			cloneAssistant(state.assistant))
		: null;
	return {
		items,
		error: state.error,
		assistant,
		reasoning: cloneReasoning(state.reasoning),
		nextId: state.nextId
	};
}

/** Reduce a chat state by one stream event. The input state is never mutated;
 *  a new ChatState is returned. */
export function reduceChatState(state: ChatState, event: StreamEvent): ChatState {
	const draft = cloneChatState(state);
	const ctx = {
		assistant: { current: draft.assistant },
		reasoning: { current: draft.reasoning }
	};
	applyEvent(draft, event, ctx);
	draft.assistant = ctx.assistant.current;
	draft.reasoning = ctx.reasoning.current;
	return draft;
}
