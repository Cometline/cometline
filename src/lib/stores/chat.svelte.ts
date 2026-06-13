import { getSessionMessages, streamMessage } from '$lib/client/cometmind';
import type { ChatItem, StreamEvent, TokenUsage, TranscriptItem } from '$lib/types';
import { reduceChatState } from '$lib/reducers/chat';
import { chatDebug, summarizeChatItems, summarizeStreamEvent } from '../debug/chat';

export type { ChatItem } from '$lib/types';

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
	let items = $state.raw<ChatItem[]>([]);
	let isLoading = $state(false);
	let isStreaming = $state(false);
	let error = $state('');
	let nextId = 0;
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
			chatDebug('store:load-transcript', {
				sessionID: nextSessionID,
				rawItems: transcript.items,
				items: summarizeChatItems(items)
			});
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
		let revealIndex = -1;
		for (let i = items.length - 1; i >= 0; i--) {
			const item = items[i];
			if (item.type === 'user' && item.reveal === false) {
				revealIndex = i;
				break;
			}
		}
		if (revealIndex < 0) return;
		items = items.map((item, i) =>
			i === revealIndex && item.type === 'user' ? { ...item, reveal: true } : item
		);
	}

	/** Shallow-copy the items array so Svelte picks up streaming updates. */
	function notifyItems() {
		items = items.slice();
	}

	function applyEvent(
		event: StreamEvent,
		ctx: {
			assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
			reasoning: { current: { text: string; pending: boolean } | null };
		}
	) {
		const reduced = reduceChatState(
			{
				items,
				error,
				assistant: ctx.assistant.current,
				reasoning: ctx.reasoning.current,
				nextId
			},
			event
		);
		items = reduced.items;
		error = reduced.error;
		ctx.assistant.current = reduced.assistant;
		ctx.reasoning.current = reduced.reasoning;
		nextId = reduced.nextId;
		notifyItems();
	}

	async function send(nextSessionID: string, text: string, opts?: { skipUser?: boolean }) {
		const run = ++streamRun;
		sessionID = nextSessionID;
		error = '';
		isStreaming = true;
		if (!opts?.skipUser) addUser(text);
		chatDebug('store:send-start', {
			sessionID: nextSessionID,
			run,
			skipUser: opts?.skipUser ?? false,
			textLength: text.length,
			items: summarizeChatItems(items)
		});
		const ctx = {
			assistant: { current: null as Extract<ChatItem, { type: 'assistant' }> | null },
			reasoning: { current: null as { text: string; pending: boolean } | null }
		};
		let eventIndex = 0;
		try {
			for await (const event of streamMessage(nextSessionID, { text })) {
				if (run !== streamRun) return;
				eventIndex += 1;
				const before = summarizeChatItems(items);
				applyEvent(event, ctx);
				chatDebug('store:stream-event', {
					sessionID: nextSessionID,
					run,
					eventIndex,
					event: summarizeStreamEvent(event),
					before,
					after: summarizeChatItems(items),
					assistantID: ctx.assistant.current?.id ?? null,
					reasoning: ctx.reasoning.current
				});
				if (event.type === 'done') break;
			}
		} catch (err) {
			if (run !== streamRun) return;
			applyEvent(
				{ type: 'error', message: err instanceof Error ? err.message : 'Failed to send message' },
				ctx
			);
		} finally {
			if (run === streamRun) {
				const beforeDone = summarizeChatItems(items);
				applyEvent({ type: 'done' }, ctx);
				isStreaming = false;
				chatDebug('store:send-finish', {
					sessionID: nextSessionID,
					run,
					beforeDone,
					afterDone: summarizeChatItems(items),
					assistantID: ctx.assistant.current?.id ?? null,
					reasoning: ctx.reasoning.current,
					error
				});
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
