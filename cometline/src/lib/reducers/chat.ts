import type { ChatItem, StreamEvent, SubagentProgressEntry } from '$lib/types';
import { isSubagentStepLimit } from '../conversation/subagent-display';
import { turnStatusLabel } from '../conversation/turn-status';
import {
	cloneReasoning as cloneReasoningSegments,
	getReasoningSegments,
	hasReasoning,
	type ReasoningSegment
} from '../conversation/reasoning';
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
	if (assistant.text.trim() || hasReasoning(assistant)) return items;
	return items.filter((item) => item.id !== assistant.id);
}

function withReasoningSegments(
	assistant: AssistantItem,
	segments: ReasoningSegment[]
): AssistantItem {
	if (segments.length === 0) {
		const { reasoning: _reasoning, ...rest } = assistant;
		return rest;
	}
	return { ...assistant, reasoning: { segments } };
}

function ensureReasoningSegments(assistant: AssistantItem): ReasoningSegment[] {
	return [...getReasoningSegments(assistant.reasoning)];
}

function pushReasoningSegment(assistant: AssistantItem): AssistantItem {
	const segments = ensureReasoningSegments(assistant);
	segments.push({ text: '', pending: true });
	return withReasoningSegments(assistant, segments);
}

function syncActiveReasoningSegment(
	assistant: AssistantItem,
	active: { text: string; pending: boolean }
): AssistantItem {
	const segments = ensureReasoningSegments(assistant);
	if (segments.length === 0) {
		segments.push({ text: active.text, pending: active.pending });
	} else {
		segments[segments.length - 1] = { text: active.text, pending: active.pending };
	}
	return withReasoningSegments(assistant, segments);
}

function finalizeActiveReasoningSegment(assistant: AssistantItem): AssistantItem {
	const segments = ensureReasoningSegments(assistant);
	if (segments.length === 0) return assistant;
	const last = segments.length - 1;
	segments[last] = { ...segments[last], pending: false };
	return withReasoningSegments(assistant, segments);
}

function finalizeAllReasoningSegments(assistant: AssistantItem): AssistantItem {
	const segments = ensureReasoningSegments(assistant).map((segment) => ({
		...segment,
		pending: false
	}));
	return withReasoningSegments(assistant, segments);
}

function currentAfterSegment(assistant: AssistantItem): number {
	return Math.max(0, ensureReasoningSegments(assistant).length - 1);
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

function appendSubagentProgress(
	progress: SubagentProgressEntry[],
	progressKind: string,
	progressText: string
): SubagentProgressEntry[] {
	const text = progressText.trim();
	if (!text) return progress;

	const next = progress.map((entry) => (entry.kind === 'stream' ? { ...entry } : { ...entry }));
	const kind = progressKind || 'message';

	if (kind === 'tool_call' || kind === 'tool_call_update' || kind === 'tool') {
		const match = text.match(/^(.+?)(?:\s+\(([^)]+)\))?$/);
		const title = (match?.[1] ?? text).trim();
		const status = (match?.[2] ?? (kind === 'tool' ? 'running' : '')).trim();
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

	if (kind === 'status' || kind === 'error') {
		const label =
			kind === 'error' ? `error: ${text.replace(/_/g, ' ')}` : text.replace(/_/g, ' ');
		const last = next[next.length - 1];
		if (last?.kind === 'status' && last.text === label) {
			return next;
		}
		next.push({ kind: 'status', text: label });
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
			reasoning: { segments: [{ text: '', pending: true }] }
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
		if (last?.type === 'assistant' && !last.text.trim() && hasReasoning(last)) {
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

	function settleTurn() {
		if (reasoning.current) reasoning.current.pending = false;
		if (assistant.current) {
			let next = assistant.current;
			if (reasoning.current) {
				next = syncActiveReasoningSegment(next, { ...reasoning.current, pending: false });
			} else {
				next = finalizeActiveReasoningSegment(next);
			}
			next = { ...next, pending: false };
			next = finalizeAllReasoningSegments(next);
			publishAssistant(next);
		}
		reasoning.current = null;
	}

	function syncReasoningPreview() {
		const host = ensureReasoningHost();
		if (!reasoning.current) return host;
		return publishAssistant(syncActiveReasoningSegment(host, reasoning.current));
	}

	function clearAssistantActivity(host: AssistantItem): AssistantItem {
		if (!host.activityPhase && !host.activityMessage) return host;
		const { activityPhase: _phase, activityMessage: _message, ...rest } = host;
		return rest;
	}

	if (event.type === 'turn_status') {
		const host = ensureAssistantForText();
		const label = turnStatusLabel(event.phase, event.message);
		publishAssistant({
			...host,
			activityPhase: event.phase,
			activityMessage: label
		});
		return;
	}

	if (event.type === 'reasoning_start') {
		reasoning.current = { text: '', pending: true };
		let host = ensureReasoningHost();
		host = clearAssistantActivity(host);
		const segments = ensureReasoningSegments(host);
		const last = segments[segments.length - 1];
		if (!(last?.pending && !last.text)) {
			host = publishAssistant(pushReasoningSegment(host));
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
		const withReasoning = host.reasoning ? finalizeAllReasoningSegments(host) : host;
		publishAssistant({
			...clearAssistantActivity(withReasoning),
			text: host.text + event.delta,
			pending: false
		});
		return;
	}

	if (event.type === 'tool_call') {
		// Settle the current assistant so reasoning is no longer pending, but keep
		// assistant.current alive so the next text_delta appends to the same turn
		// instead of creating a fresh assistant row (which would lose its avatar).
		settleTurn();
		reasoning.current = null;
		if (assistant.current) {
			assistant.current = clearAssistantActivity(assistant.current);
		}
		const afterSegment = assistant.current ? currentAfterSegment(assistant.current) : 0;
		const id = localID('tool', draft.nextId++).id;
		items.push({
			id,
			type: 'tool',
			toolId: event.id,
			toolName: event.tool,
			input: event.input,
			pending: true,
			startedAt: Date.now(),
			afterSegment
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
		settleTurn();
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
			let status: Extract<ChatItem, { type: 'subagent' }>['status'] =
				event.delegation_status === 'completed'
					? 'completed'
					: event.delegation_status === 'cancelled'
						? 'cancelled'
						: 'failed';
			const summary = event.summary;
			const finishedCard: Extract<ChatItem, { type: 'subagent' }> = {
				...card,
				status,
				summary,
				pending: false
			};
			if (status === 'failed' && isSubagentStepLimit(finishedCard)) {
				status = 'incomplete';
			}
			items[index] = {
				...finishedCard,
				status
			};
		}
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
		settleTurn();
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
		settleTurn();
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
		reasoning: cloneReasoningSegments(a.reasoning),
		memoryUpdates: a.memoryUpdates?.map((update) => ({ ...update }))
	};
}

function cloneItem(item: ChatItem): ChatItem {
	if (item.type === 'user') {
		return { ...item, reveal: item.reveal ?? true };
	}
	if (item.type === 'assistant') {
		return cloneAssistant(item)!;
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
