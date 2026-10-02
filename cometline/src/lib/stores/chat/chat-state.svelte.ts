import type { ChatItem } from '#lib/types.js';
import type { ContextBudgetSnapshot } from '#lib/context-window.js';
import type { SessionStream } from '#lib/stores/chat-stream-types.js';

export type TranscriptPageState = { hasMore: boolean; nextBefore: string; olderPageSeq: number };

export function createChatState() {
	let sessionID = $state<string | null>(null);
	let items = $state.raw<ChatItem[]>([]);
	let isLoading = $state(false);
	let error = $state('');
	let isLoadingOlder = $state(false);
	let hasMoreHistory = $state(false);
	let streamingSessionIds = $state.raw<Set<string>>(new Set());
	let failedRunSessionIds = $state.raw<Set<string>>(new Set());
	let contextBudget = $state.raw<ContextBudgetSnapshot | null>(null);

	return {
		get sessionID() {
			return sessionID;
		},
		set sessionID(value) {
			sessionID = value;
		},
		get items() {
			return items;
		},
		set items(value) {
			items = value;
		},
		get isLoading() {
			return isLoading;
		},
		set isLoading(value) {
			isLoading = value;
		},
		get error() {
			return error;
		},
		set error(value) {
			error = value;
		},
		get isLoadingOlder() {
			return isLoadingOlder;
		},
		set isLoadingOlder(value) {
			isLoadingOlder = value;
		},
		get hasMoreHistory() {
			return hasMoreHistory;
		},
		set hasMoreHistory(value) {
			hasMoreHistory = value;
		},
		get streamingSessionIds() {
			return streamingSessionIds;
		},
		set streamingSessionIds(value) {
			streamingSessionIds = value;
		},
		get failedRunSessionIds() {
			return failedRunSessionIds;
		},
		set failedRunSessionIds(value) {
			failedRunSessionIds = value;
		},
		get contextBudget() {
			return contextBudget;
		},
		set contextBudget(value) {
			contextBudget = value;
		},
		nextId: 0,
		globalStreamRun: 0,
		loadRun: 0,
		loadPromise: null as Promise<void> | null,
		loadPromiseSession: null as string | null,
		sessionCache: new Map<string, ChatItem[]>(),
		sessionErrors: new Map<string, string>(),
		sessionContextBudgets: new Map<string, ContextBudgetSnapshot>(),
		sessionTranscriptPages: new Map<string, TranscriptPageState>(),
		streamHandles: new Map<string, SessionStream>(),
		localStreamingSessionIds: new Set<string>(),
		remoteStreamingSessionIds: new Set<string>(),
		pendingChatItemsBroadcast: new Map<string, ChatItem[]>(),
		chatItemsBroadcastTimers: new Map<string, ReturnType<typeof setTimeout>>()
	};
}

export type ChatState = ReturnType<typeof createChatState>;
