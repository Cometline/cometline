import { getSessionMessages, streamMessage } from '$lib/client/cometmind';
import type { StreamEvent, TokenUsage, TranscriptItem } from '$lib/types';

export type ChatItem =
	| { id: string; type: 'user'; text: string; reveal?: boolean }
	| {
			id: string;
			type: 'assistant';
			text: string;
			pending?: boolean;
			reasoning?: { text: string; pending?: boolean };
	  }
	| {
			id: string;
			type: 'tool';
			toolId?: string;
			toolName: string;
			input: unknown;
			output?: string;
			error?: string;
			pending?: boolean;
	  }
	| { id: string; type: 'status'; text: string; usage?: TokenUsage }
	| { id: string; type: 'error'; text: string };

let nextLocalID = 0;

function localID(prefix: string) {
	nextLocalID += 1;
	return `${prefix}-${Date.now()}-${nextLocalID}`;
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

function itemsFromTranscript(transcriptItems: TranscriptItem[]): ChatItem[] {
	const out: ChatItem[] = [];
	for (let i = 0; i < transcriptItems.length; i++) {
		const item = transcriptItems[i];
		if (item.type === 'reasoning') {
			const next = transcriptItems[i + 1];
			if (next?.type === 'assistant') {
				out.push({
					id: `history-${i}`,
					type: 'assistant',
					text: next.text,
					reasoning: { text: item.text, pending: false }
				});
				i++;
				continue;
			}
		}
		if (item.type === 'assistant') {
			const next = transcriptItems[i + 1];
			if (next?.type === 'reasoning') {
				out.push({
					id: `history-${i}`,
					type: 'assistant',
					text: item.text,
					reasoning: { text: next.text, pending: false }
				});
				i++;
				continue;
			}
			const prev = transcriptItems[i - 1];
			if (prev?.type === 'reasoning') continue;
		}
		out.push(itemFromTranscript(item, i));
	}
	return out;
}

function itemFromTranscript(item: TranscriptItem, index: number): ChatItem {
	if (item.type === 'user') return { id: `history-${index}`, type: 'user', text: item.text };
	if (item.type === 'assistant') return { id: `history-${index}`, type: 'assistant', text: item.text };
	if (item.type === 'reasoning') return { id: `history-${index}`, type: 'assistant', text: '', reasoning: { text: item.text, pending: false } };
	return {
		id: `history-${index}`,
		type: 'tool',
		toolName: item.tool_name,
		input: item.tool_input,
		output: item.tool_output,
		error: item.tool_error ? item.tool_output : undefined,
		pending: false
	};
}

function createChatStore() {
	let sessionID = $state<string | null>(null);
	let items = $state<ChatItem[]>([]);
	let isLoading = $state(false);
	let isStreaming = $state(false);
	let error = $state('');
	let streamRun = 0;
	let loadRun = 0;

	function clear() {
		sessionID = null;
		items = [];
		isLoading = false;
		isStreaming = false;
		error = '';
		streamRun += 1;
		loadRun += 1;
	}

	async function loadTranscript(nextSessionID: string) {
		if (sessionID === nextSessionID && items.length > 0) return;
		if (isStreaming && sessionID === nextSessionID) return;

		const run = ++loadRun;
		const switchingSession = sessionID !== nextSessionID;
		sessionID = nextSessionID;
		if (switchingSession) items = [];
		isLoading = true;
		error = '';
		try {
			const transcript = await getSessionMessages(nextSessionID);
			if (run !== loadRun) return;
			if (isStreaming && sessionID === nextSessionID) return;
			items = itemsFromTranscript(transcript.items);
		} catch (err) {
			if (run !== loadRun) return;
			if (isStreaming && sessionID === nextSessionID) return;
			error = err instanceof Error ? err.message : 'Failed to load transcript';
			items = [{ id: localID('error'), type: 'error', text: error }];
		} finally {
			if (run === loadRun) isLoading = false;
		}
	}

	function addUser(text: string, reveal = true) {
		items.push({ id: localID('user'), type: 'user', text, reveal });
		notifyItems();
	}

	function stageUser(text: string) {
		addUser(text, false);
	}

	function revealStagedUser() {
		const staged = [...items].reverse().find((item) => item.type === 'user' && item.reveal === false);
		if (!staged || staged.type !== 'user') return;
		staged.reveal = true;
		notifyItems();
	}

	function findTool(toolId: string) {
		return items.find((item) => item.type === 'tool' && item.toolId === toolId);
	}

	/** Shallow-copy the items array so Svelte picks up in-place mutations during streaming. */
	function notifyItems() {
		items = items.slice();
	}

	function removeEmptyAssistant(assistant: Extract<ChatItem, { type: 'assistant' }>) {
		if (assistant.text.trim() || assistant.reasoning?.text.trim()) return;
		items = items.filter((item) => item.id !== assistant.id);
	}

	function attachReasoning(
		assistant: Extract<ChatItem, { type: 'assistant' }>,
		reasoning: { text: string; pending: boolean } | null
	) {
		if (!reasoning || (!reasoning.text.trim() && !reasoning.pending)) return;
		const chunk = reasoning.text;
		if (assistant.reasoning?.text) {
			assistant.reasoning.text += `${assistant.reasoning.text ? '\n\n' : ''}${chunk}`;
			assistant.reasoning.pending = reasoning.pending;
		} else {
			assistant.reasoning = { text: chunk, pending: reasoning.pending };
		}
	}

	function finalizeReasoning(
		assistant: Extract<ChatItem, { type: 'assistant' }> | null,
		reasoning: { text: string; pending: boolean } | null
	) {
		if (reasoning) reasoning.pending = false;
		if (assistant?.reasoning) assistant.reasoning.pending = false;
	}

	function flushReasoningToAssistant(
		assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null },
		reasoning: { current: { text: string; pending: boolean } | null }
	) {
		if (!assistant.current || !reasoning.current) return;
		if (assistant.current.reasoning) {
			assistant.current.reasoning.pending = reasoning.current.pending;
		} else {
			attachReasoning(assistant.current, reasoning.current);
		}
		reasoning.current = null;
	}

	function settleTurn(ctx: {
		assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
		reasoning: { current: { text: string; pending: boolean } | null };
	}) {
		finalizeReasoning(ctx.assistant.current, ctx.reasoning.current);
		flushReasoningToAssistant(ctx.assistant, ctx.reasoning);
		if (ctx.assistant.current) {
			ctx.assistant.current.pending = false;
			if (ctx.assistant.current.reasoning) ctx.assistant.current.reasoning.pending = false;
		}
	}

	function applyEvent(
		event: StreamEvent,
		ctx: {
			assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
			reasoning: { current: { text: string; pending: boolean } | null };
		}
	) {
		const { assistant, reasoning } = ctx;

		function pushAssistant(next: Extract<ChatItem, { type: 'assistant' }>) {
			// Push then re-read the element so we hold the reactive $state proxy,
			// not the raw object. Mutating the raw reference does not trigger
			// Svelte 5 fine-grained reactivity, so streamed deltas would never
			// render until the transcript was reloaded.
			const index = items.push(next) - 1;
			const proxy = items[index] as Extract<ChatItem, { type: 'assistant' }>;
			assistant.current = proxy;
			return proxy;
		}

		function ensureReasoningHost() {
			if (assistant.current) return assistant.current;
			return pushAssistant({
				id: localID('assistant'),
				type: 'assistant',
				text: '',
				reasoning: { text: '', pending: true }
			});
		}

		function ensureAssistantForText() {
			if (assistant.current) return assistant.current;
			return pushAssistant({ id: localID('assistant'), type: 'assistant', text: '' });
		}

		function clearEmptyAssistant() {
			if (!assistant.current) return;
			removeEmptyAssistant(assistant.current);
			assistant.current = null;
		}

		function ensureTurnReasoning() {
			if (!reasoning.current) reasoning.current = { text: '', pending: true };
			return reasoning.current;
		}

		function syncReasoningPreview() {
			if (!reasoning.current) return ensureReasoningHost();
			const host = ensureReasoningHost();
			host.reasoning = {
				text: reasoning.current.text,
				pending: reasoning.current.pending
			};
			return host;
		}

		try {
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
				settleTurn(ctx);
				clearEmptyAssistant();
				items.push({
					id: localID('tool'),
					type: 'tool',
					toolId: event.id,
					toolName: event.tool,
					input: event.input,
					pending: true
				});
				return;
			}
			if (event.type === 'tool_result') {
				const tool = findTool(event.id);
				if (tool?.type === 'tool') {
					tool.output = event.output;
					tool.error = event.error;
					tool.pending = false;
				}
				return;
			}
			if (event.type === 'step_finish') {
				settleTurn(ctx);
				if (assistant.current && !assistant.current.text.trim()) {
					clearEmptyAssistant();
				}
				return;
			}
			if (event.type === 'error') {
				settleTurn(ctx);
				clearEmptyAssistant();
				error = cleanErrorMessage(event.message);
				items.push({ id: localID('error'), type: 'error', text: error });
				return;
			}
			if (event.type === 'done') {
				settleTurn(ctx);
				if (assistant.current && !assistant.current.text.trim()) {
					clearEmptyAssistant();
				}
			}
		} finally {
			notifyItems();
		}
	}

	async function send(nextSessionID: string, text: string, opts?: { skipUser?: boolean }) {
		const run = ++streamRun;
		sessionID = nextSessionID;
		error = '';
		isStreaming = true;
		if (!opts?.skipUser) addUser(text);
		const ctx = {
			assistant: { current: null as Extract<ChatItem, { type: 'assistant' }> | null },
			reasoning: { current: null as { text: string; pending: boolean } | null }
		};
		try {
			for await (const event of streamMessage(nextSessionID, { text })) {
				if (run !== streamRun) return;
				applyEvent(event, ctx);
				if (event.type === 'done') break;
			}
		} catch (err) {
			if (run !== streamRun) return;
			error = cleanErrorMessage(err instanceof Error ? err.message : 'Failed to send message');
			settleTurn(ctx);
			if (ctx.assistant.current) {
				removeEmptyAssistant(ctx.assistant.current);
				ctx.assistant.current = null;
			}
			items.push({ id: localID('error'), type: 'error', text: error });
		} finally {
			if (run === streamRun) {
				settleTurn(ctx);
				if (ctx.assistant.current && !ctx.assistant.current.text.trim()) {
					removeEmptyAssistant(ctx.assistant.current);
				}
				isStreaming = false;
				notifyItems();
			}
		}
	}

	return {
		get sessionID() {
			return sessionID;
		},
		get items() {
			return items;
		},
		get isLoading() {
			return isLoading;
		},
		get isStreaming() {
			return isStreaming;
		},
		get error() {
			return error;
		},
		clear,
		loadTranscript,
		stageUser,
		revealStagedUser,
		send
	};
}

export const chatStore = createChatStore();
