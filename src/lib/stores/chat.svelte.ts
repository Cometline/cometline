import { abortSession, getSessionMessages, listChildSessions, respondToSubagent, streamMessage } from '$lib/client/cometmind';
import type { ChatItem, ImageAttachment, Session, StreamEvent, TranscriptItem } from '$lib/types';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import { reduceChatState } from '$lib/reducers/chat';
import { chatDebug, summarizeChatItems, summarizeStreamEvent } from '../debug/chat';

export type { ChatItem } from '$lib/types';

let nextLocalID = 0;

function localID(prefix: string) {
	nextLocalID += 1;
	return `${prefix}-${Date.now()}-${nextLocalID}`;
}

function mapDelegationStatus(
	status: string | undefined
): Extract<ChatItem, { type: 'subagent' }>['status'] {
	switch (status) {
		case 'completed':
			return 'completed';
		case 'cancelled':
			return 'cancelled';
		case 'failed':
			return 'failed';
		case 'running':
			return 'running';
		case 'awaiting_user':
			return 'awaiting_user';
		case 'awaiting_permission':
			return 'awaiting_permission';
		default:
			return 'pending';
	}
}

function subagentFromChild(child: Session, agentName = 'opencode'): Extract<ChatItem, { type: 'subagent' }> {
	return {
		id: `subagent-${child.id}`,
		type: 'subagent',
		childSessionId: child.id,
		purpose: child.purpose ?? child.title ?? 'Delegated coding task',
		agentName,
		status: mapDelegationStatus(child.delegation_status),
		progress: [],
		summary: child.output_summary,
		pending: child.delegation_status === 'running' || child.delegation_status === 'awaiting_user' || child.delegation_status === 'awaiting_permission',
		pendingQuestion: child.pending_question || undefined
	};
}

function mergeSubagents(items: ChatItem[], children: Session[]): ChatItem[] {
	if (children.length === 0) return items;
	const used = new Set<string>();
	const out: ChatItem[] = [];

	for (const item of items) {
		if (item.type === 'tool' && item.toolName === 'delegate_coding_task') {
			const match = children.find(
				(child) => !used.has(child.id) && item.output?.includes(child.id)
			);
			if (match) {
				used.add(match.id);
				out.push(subagentFromChild(match));
				continue;
			}
		}
		out.push(item);
	}

	for (const child of children) {
		if (!used.has(child.id)) {
			out.push(subagentFromChild(child));
		}
	}
	return out;
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
					text: next.text ?? '',
					reasoning: { text: item.text ?? '', pending: false }
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
					text: item.text ?? '',
					reasoning: { text: next.text ?? '', pending: false }
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
	if (item.type === 'user')
		return { id: `history-${index}`, type: 'user', text: item.text ?? '', images: item.images };
	if (item.type === 'assistant')
		return { id: `history-${index}`, type: 'assistant', text: item.text ?? '' };
	if (item.type === 'system')
		return { id: `history-${index}`, type: 'status', text: item.text ?? '' };
	if (item.type === 'reasoning')
		return {
			id: `history-${index}`,
			type: 'assistant',
			text: '',
			reasoning: { text: item.text ?? '', pending: false }
		};
	return {
		id: `history-${index}`,
		type: 'tool',
		toolName: item.tool_name ?? '',
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
	let streamAbort: AbortController | null = null;
	let loadPromise: Promise<void> | null = null;
	let loadPromiseSession: string | null = null;

	function isAbortError(err: unknown) {
		return err instanceof DOMException && err.name === 'AbortError';
	}

	function clear() {
		sessionID = null;
		items = [];
		isLoading = false;
		isStreaming = false;
		error = '';
		streamRun += 1;
		loadRun += 1;
		loadPromise = null;
		loadPromiseSession = null;
		streamAbort?.abort();
		streamAbort = null;
	}

	function bindSession(nextSessionID: string) {
		if (sessionID === nextSessionID) return;
		loadRun += 1;
		loadPromise = null;
		loadPromiseSession = null;
		sessionID = nextSessionID;
		items = [];
		isLoading = false;
		error = '';
	}

	async function loadTranscript(nextSessionID: string) {
		if (sessionID === nextSessionID && items.length > 0) return;
		if (isStreaming && sessionID === nextSessionID) return;
		if (sessionID === nextSessionID && isLoading && loadPromise) return loadPromise;

		const run = ++loadRun;
		const switchingSession = sessionID !== nextSessionID;
		sessionID = nextSessionID;
		if (switchingSession) items = [];
		isLoading = true;
		error = '';
		loadPromiseSession = nextSessionID;
		loadPromise = (async () => {
			try {
				const [transcript, children] = await Promise.all([
					getSessionMessages(nextSessionID),
					listChildSessions(nextSessionID).catch(() => ({ sessions: [] as Session[] }))
				]);
				if (run !== loadRun) return;
				if (isStreaming && sessionID === nextSessionID) return;
				// First-turn flight may stage a user message while this fetch was in flight.
				if (sessionID === nextSessionID && items.length > 0) return;
				items = mergeSubagents(itemsFromTranscript(transcript.items), children.sessions);
				chatDebug('store:load-transcript', {
					sessionID: nextSessionID,
					rawItems: transcript.items,
					items: summarizeChatItems(items)
				});
			} catch (err) {
				if (run !== loadRun) return;
				if (isStreaming && sessionID === nextSessionID) return;
				if (sessionID === nextSessionID && items.length > 0) return;
				error = err instanceof Error ? err.message : 'Failed to load transcript';
				items = [{ id: localID('error'), type: 'error', text: error }];
			} finally {
				if (run === loadRun) {
					isLoading = false;
					if (loadPromiseSession === nextSessionID) {
						loadPromise = null;
						loadPromiseSession = null;
					}
				}
			}
		})();
		return loadPromise;
	}

	function addUser(text: string, images?: ImageAttachment[], reveal = true) {
		items.push({ id: localID('user'), type: 'user', text, images, reveal });
		notifyItems();
	}

	function stageUser(text: string, images?: ImageAttachment[]) {
		addUser(text, images, false);
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

	/** Publish items array reference for Svelte reactivity (caller may already have a new array). */
	function notifyItems(alreadyNew = false) {
		if (!alreadyNew) items = items.slice();
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
		notifyItems(true);
	}

	const BATCHABLE_EVENTS = new Set(['text_delta', 'reasoning_delta', 'reasoning_start']);
	let pendingBatchEvents: StreamEvent[] = [];
	let batchFrame = 0;

	function flushBatch(
		ctx: {
			assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
			reasoning: { current: { text: string; pending: boolean } | null };
		}
	) {
		if (pendingBatchEvents.length === 0) return;
		const batch = pendingBatchEvents;
		pendingBatchEvents = [];
		for (const event of batch) {
			applyEvent(event, ctx);
		}
	}

	function scheduleBatch(
		event: StreamEvent,
		ctx: {
			assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
			reasoning: { current: { text: string; pending: boolean } | null };
		}
	) {
		pendingBatchEvents.push(event);
		if (batchFrame) return;
		batchFrame = requestAnimationFrame(() => {
			batchFrame = 0;
			flushBatch(ctx);
		});
	}

	function applyStreamEvent(
		event: StreamEvent,
		ctx: {
			assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
			reasoning: { current: { text: string; pending: boolean } | null };
		}
	) {
		if (BATCHABLE_EVENTS.has(event.type)) {
			scheduleBatch(event, ctx);
			return;
		}
		if (pendingBatchEvents.length > 0) {
			flushBatch(ctx);
		}
		applyEvent(event, ctx);
	}

	async function send(
		nextSessionID: string,
		payloadOrText: ChatTurnPayload | string,
		opts?: { skipUser?: boolean }
	) {
		const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
		const text = payload.text;
		const images = payload.images;
		if (isStreaming) {
			chatDebug('store:send-blocked', {
				sessionID: nextSessionID,
				reason: 'already-streaming',
				textLength: text.length
			});
			return;
		}

		const run = ++streamRun;
		sessionID = nextSessionID;
		error = '';
		isStreaming = true;
		streamAbort?.abort();
		streamAbort = new AbortController();
		const signal = streamAbort.signal;
		if (!opts?.skipUser) addUser(text, images);
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
			for await (const event of streamMessage(
				nextSessionID,
				{
					text,
					images: images?.map((image) => ({ media_type: image.media_type, data: image.data })),
					file_paths: payload.filePaths
				},
				signal
			)) {
				if (run !== streamRun) return;
				eventIndex += 1;
				const before = summarizeChatItems(items);
				applyStreamEvent(event, ctx);
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
			if (isAbortError(err)) {
				chatDebug('store:send-aborted', { sessionID: nextSessionID, run });
				return;
			}
			applyStreamEvent(
				{
					type: 'error',
					message: err instanceof Error ? err.message : 'Failed to send message'
				},
				ctx
			);
		} finally {
			if (run === streamRun) {
				if (pendingBatchEvents.length > 0) {
					flushBatch(ctx);
				}
				const beforeDone = summarizeChatItems(items);
				applyEvent({ type: 'done' }, ctx);
				isStreaming = false;
				streamAbort = null;
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

	async function cancel(nextSessionID?: string) {
		const id = nextSessionID ?? sessionID;
		if (!id || !isStreaming) return;

		chatDebug('store:cancel-start', { sessionID: id });
		streamAbort?.abort();
		try {
			await abortSession(id);
		} catch (err) {
			chatDebug('store:cancel-abort-failed', {
				sessionID: id,
				error: err instanceof Error ? err.message : String(err)
			});
		}
	}

	function patchSubagentCard(
		childSessionId: string,
		patch: Partial<Extract<ChatItem, { type: 'subagent' }>>
	) {
		items = items.map((item) =>
			item.type === 'subagent' && item.childSessionId === childSessionId ? { ...item, ...patch } : item
		);
	}

	async function replyToSubagent(childSessionId: string, text: string, permissionOptionId?: string) {
		patchSubagentCard(childSessionId, {
			status: 'running',
			pending: true,
			pendingQuestion: undefined,
			permissionOptions: undefined
		});
		try {
			for await (const event of respondToSubagent(childSessionId, {
				text: text || undefined,
				permission_option_id: permissionOptionId
			})) {
				const ctx = {
					assistant: { current: null as Extract<ChatItem, { type: 'assistant' }> | null },
					reasoning: { current: null as { text: string; pending: boolean } | null }
				};
				applyStreamEvent(event, ctx);
				if (event.type === 'done') break;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reply to subagent';
		}
	}

	async function cancelSubagent(childSessionId: string) {
		try {
			await abortSession(childSessionId);
			patchSubagentCard(childSessionId, {
				status: 'cancelled',
				pending: false,
				pendingQuestion: undefined,
				permissionOptions: undefined
			});
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to cancel subagent';
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
		bindSession,
		loadTranscript,
		stageUser,
		revealStagedUser,
		send,
		cancel,
		replyToSubagent,
		cancelSubagent
	};
}

export const chatStore = createChatStore();
