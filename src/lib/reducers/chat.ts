import type { ChatItem, StreamEvent, SubagentProgressEntry } from '$lib/types';
import {
	chatDebug,
	summarizeChatItem,
	summarizeChatItems,
	summarizeStreamEvent
} from '../debug/chat';

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
	if (text.includes('authentication failed') || text.includes('HTTP 401')) {
		return 'API key is invalid or missing. Open Settings (⌘,), enter your provider API key, and click Save.';
	}
	if (text.includes('Client.Timeout exceeded while awaiting headers')) {
		return 'The model provider did not start responding before the request timed out. This is usually a provider queue, gateway, or model availability issue. Try again, or switch provider/model if it keeps happening.';
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
			reasoning: {
				text: assistant.reasoning.text + '\n\n' + chunk,
				pending: reasoning.pending
			}
		};
	}
	return { ...assistant, reasoning: { text: chunk, pending: reasoning.pending } };
}

function settlePendingTools(items: ChatItem[]) {
	for (let i = 0; i < items.length; i++) {
		const item = items[i];
		if (item.type !== 'tool' || !item.pending) continue;
		items[i] = {
			...item,
			pending: false,
			durationMs: item.startedAt != null ? Date.now() - item.startedAt : item.durationMs
		};
	}
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

function appendSubagentProgress(
	progress: SubagentProgressEntry[],
	progressKind: string,
	progressText: string
): SubagentProgressEntry[] {
	const text = progressText.trim();
	if (!text) return progress;

	const next = progress.map((entry) =>
		entry.kind === 'stream' ? { ...entry } : { ...entry }
	);
	const kind = progressKind || 'message';

	if (kind === 'tool_call' || kind === 'tool_call_update') {
		const match = text.match(/^(.+?)(?:\s+\(([^)]+)\))?$/);
		const title = (match?.[1] ?? text).trim();
		const status = (match?.[2] ?? '').trim();
		const index = next.findIndex((entry) => entry.kind === 'tool' && entry.title === title);
		if (index >= 0) {
			const existing = next[index];
			if (existing.kind === 'tool') {
				next[index] = {
					kind: 'tool',
					title: existing.title,
					status: status || existing.status
				};
			}
		} else {
			next.push({ kind: 'tool', title, status });
		}
		return next;
	}

	const channel =
		kind === 'thought' ? 'thought' : kind === 'plan' ? 'plan' : ('message' as const);
	for (let i = next.length - 1; i >= 0; i--) {
		const entry = next[i];
		if (entry.kind === 'stream' && entry.channel === channel) {
			entry.text += progressText;
			return next;
		}
	}
	next.push({ kind: 'stream', channel, text: progressText });
	return next;
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
	const { items } = draft;

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
		if (assistant.current) {
			chatDebug('reducer:assistant-host', {
				choice: 'current',
				event: summarizeStreamEvent(event),
				assistant: summarizeChatItem(assistant.current)
			});
			return assistant.current;
		}
		const last = items[items.length - 1];
		if (last?.type === 'assistant' && !last.text.trim() && last.reasoning?.text.trim()) {
			assistant.current = last;
			chatDebug('reducer:assistant-host', {
				choice: 'reuse-last-reasoning-only',
				event: summarizeStreamEvent(event),
				assistant: summarizeChatItem(last),
				items: summarizeChatItems(items)
			});
			return last;
		}
		const id = localID('assistant', draft.nextId++).id;
		const next: AssistantItem = { id, type: 'assistant', text: '' };
		pushAssistant(next);
		chatDebug('reducer:assistant-host', {
			choice: 'new',
			event: summarizeStreamEvent(event),
			assistant: summarizeChatItem(next),
			items: summarizeChatItems(items)
		});
		return next;
	}

	function clearEmptyAssistant() {
		if (!assistant.current) return;
		draft.items = removeEmptyAssistant(draft.items, assistant.current);
		assistant.current = null;
	}

	function ensureTurnReasoning() {
		if (!reasoning.current) reasoning.current = { text: '', pending: true };
		return reasoning.current;
	}

	function publishAssistant(next: AssistantItem) {
		const index = items.findIndex((item) => item.id === next.id);
		if (index >= 0) {
			items[index] = next;
		}
		assistant.current = next;
		return next;
	}

	function syncReasoningPreview() {
		const host = ensureReasoningHost();
		if (!reasoning.current) return host;
		return publishAssistant({
			...host,
			pending: true,
			reasoning: { text: reasoning.current.text, pending: reasoning.current.pending }
		});
	}

	if (event.type === 'reasoning_start') {
		if (reasoning.current?.text) {
			reasoning.current.text += '\n\n';
		} else {
			reasoning.current = { text: '', pending: true };
		}
		syncReasoningPreview();
		return;
	}

	if (event.type === 'reasoning_delta') {
		const turnReasoning = ensureTurnReasoning();
		turnReasoning.text += event.text;
		syncReasoningPreview();
		return;
	}

	if (event.type === 'text_delta') {
		const host = ensureAssistantForText();
		if (reasoning.current) reasoning.current.pending = false;
		reasoning.current = null;
		publishAssistant({
			...host,
			text: host.text + event.delta,
			pending: false,
			reasoning: host.reasoning ? { ...host.reasoning, pending: false } : undefined
		});
		return;
	}

	if (event.type === 'tool_call') {
		// Settle the current assistant so reasoning is no longer pending, but keep
		// assistant.current alive so the next text_delta appends to the same turn
		// instead of creating a fresh assistant row (which would lose its avatar).
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		reasoning.current = null;
		const id = localID('tool', draft.nextId++).id;
		items.push({
			id,
			type: 'tool',
			toolId: event.id,
			toolName: event.tool,
			input: event.input,
			pending: true,
			startedAt: Date.now()
		});
		return;
	}

	if (event.type === 'tool_result') {
		const tool = items.find((item) => item.type === 'tool' && item.toolId === event.id) as
			| Extract<ChatItem, { type: 'tool' }>
			| undefined;
		if (tool) {
			const index = items.indexOf(tool);
			items[index] = {
				...tool,
				output: event.error ? undefined : event.output,
				error: event.error || undefined,
				pending: false,
				durationMs: tool.startedAt != null ? Date.now() - tool.startedAt : tool.durationMs
			};
		}
		return;
	}

	if (event.type === 'step_finish') {
		// Settle reasoning/assistant state without clearing assistant.current so a
		// multi-step turn keeps streaming into one assistant bubble.
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		reasoning.current = null;
		return;
	}

	if (event.type === 'subagent_started') {
		const id = localID('subagent', draft.nextId++).id;
		items.push({
			id,
			type: 'subagent',
			childSessionId: event.child_session_id,
			purpose: event.purpose,
			agentName: event.agent_name,
			status: 'running',
			progress: [],
			pending: true
		});
		return;
	}

	if (event.type === 'subagent_progress') {
		const card = items.find(
			(item) => item.type === 'subagent' && item.childSessionId === event.child_session_id
		) as Extract<ChatItem, { type: 'subagent' }> | undefined;
		if (card && event.progress_text) {
			const index = items.indexOf(card);
			items[index] = {
				...card,
				progress: appendSubagentProgress(
					card.progress,
					event.progress_kind,
					event.progress_text
				)
			};
		}
		return;
	}

	if (event.type === 'subagent_finished') {
		const card = items.find(
			(item) => item.type === 'subagent' && item.childSessionId === event.child_session_id
		) as Extract<ChatItem, { type: 'subagent' }> | undefined;
		if (card) {
			const index = items.indexOf(card);
			const status =
				event.delegation_status === 'completed'
					? 'completed'
					: event.delegation_status === 'cancelled'
						? 'cancelled'
						: 'failed';
			items[index] = {
				...card,
				status,
				summary: event.summary,
				pending: false,
				pendingQuestion: undefined,
				permissionOptions: undefined
			};
		}
		return;
	}

	if (event.type === 'subagent_awaiting_input') {
		if (event.kind === 'resumed') return;
		const card = items.find(
			(item) => item.type === 'subagent' && item.childSessionId === event.child_session_id
		) as Extract<ChatItem, { type: 'subagent' }> | undefined;
		if (!card) return;
		const index = items.indexOf(card);
		const status = event.kind === 'permission' ? 'awaiting_permission' : 'awaiting_user';
		items[index] = {
			...card,
			status,
			pending: true,
			pendingQuestion: event.question,
			permissionOptions: event.permission_options
		};
		return;
	}

	if (event.type === 'memory_injected') {
		const id = localID('memory', draft.nextId++).id;
		items.push({
			id,
			type: 'memory',
			memories: event.memories
		});
		return;
	}

	if (event.type === 'memory_updated') {
		if (!assistant.current) return;
		const index = items.findIndex((item) => item.id === assistant.current!.id);
		if (index < 0) return;
		const current = items[index] as AssistantItem;
		const next: AssistantItem = {
			...current,
			memoryUpdates: [...(current.memoryUpdates ?? []), ...event.changes]
		};
		items[index] = next;
		assistant.current = next;
		return;
	}

	if (event.type === 'error') {
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		// clearEmptyAssistant() reassigns draft.items to a new array, so the local
		// `items` reference captured at the top of applyEvent becomes stale. Push
		// the error onto the live draft.items array (not the orphaned `items` one),
		// otherwise the error card silently never renders.
		clearEmptyAssistant();
		settlePendingTools(draft.items);
		draft.error = cleanErrorMessage(event.message);
		const id = localID('error', draft.nextId++).id;
		draft.items.push({ id, type: 'error', text: draft.error });
		return;
	}

	if (event.type === 'done') {
		settleTurn({ assistant: assistant.current, reasoning: reasoning.current });
		settlePendingTools(items);
		if (assistant.current && !assistant.current.text.trim()) {
			clearEmptyAssistant();
		}
	}
}

function cloneReasoning(
	r: { text: string; pending: boolean } | null
): { text: string; pending: boolean } | null {
	return r ? { text: r.text, pending: r.pending } : null;
}

function cloneAssistant(a: AssistantItem | null): AssistantItem | null {
	if (!a) return null;
	return {
		...a,
		reasoning: a.reasoning
			? { text: a.reasoning.text, pending: a.reasoning.pending }
			: undefined
	};
}

function cloneItem(item: ChatItem): ChatItem {
	if (item.type === 'user') {
		return { ...item, reveal: item.reveal ?? true };
	}
	if (item.type === 'assistant') {
		return {
			...item,
			reasoning: item.reasoning
				? { text: item.reasoning.text, pending: item.reasoning.pending }
				: undefined,
			memoryUpdates: item.memoryUpdates?.map((update) => ({ ...update }))
		};
	}
	if (item.type === 'subagent') {
		return {
			...item,
			progress: item.progress.map((entry) =>
				entry.kind === 'stream' ? { ...entry } : { ...entry }
			)
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

function isDeltaOnlyEvent(event: StreamEvent): boolean {
	return (
		event.type === 'text_delta' ||
		event.type === 'reasoning_delta' ||
		event.type === 'reasoning_start' ||
		event.type === 'step_finish'
	);
}

/** Shallow-copy items array only; mutates assistant/reasoning in place for streaming deltas. */
function reduceChatStateDelta(state: ChatState, event: StreamEvent): ChatState {
	const items = state.items.slice();
	const draft: ChatState = {
		items,
		error: state.error,
		assistant: state.assistant,
		reasoning: state.reasoning ? { ...state.reasoning } : null,
		nextId: state.nextId
	};
	const ctx = {
		assistant: { current: draft.assistant },
		reasoning: { current: draft.reasoning }
	};
	applyEvent(draft, event, ctx);
	draft.assistant = ctx.assistant.current;
	draft.reasoning = ctx.reasoning.current;
	return draft;
}

/** Reduce a chat state by one stream event. The input state is never mutated;
 *  a new ChatState is returned. */
export function reduceChatState(state: ChatState, event: StreamEvent): ChatState {
	if (isDeltaOnlyEvent(event)) {
		return reduceChatStateDelta(state, event);
	}
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
