import { untrack } from 'svelte';
import type { ChatItem } from '$lib/stores/chat.svelte';
import { chatStore } from '$lib/stores/chat.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

export interface SessionPhaseDeps {
	getSessionId: () => string;
	/** Abort in-flight bubble flights. DOM lives outside this module. */
	abortFlights: () => void;
	syncQueueState: () => void;
	syncComposerPhase: (opts: {
		hasVisibleConversation: boolean;
		firstTurnActive: boolean;
		awaitingFirstAssistant: boolean;
	}) => void;
}

function hasTurns(list: readonly ChatItem[]) {
	return list.some((item) => item.type === 'user' || item.type === 'assistant');
}

/**
 * Session phase owns the flags ChatView used to synchronise with $effect.
 * Interface is phase + onSession. Snapshot copy exists only for the soft-swap
 * gap while chatStore is still bound to the previous session.
 */
export function createSessionPhase(deps: SessionPhaseDeps) {
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);
	let firstTurnHandoffPending = $state(false);
	let awaitingFirstAssistant = $state(false);
	let snapshotItems = $state.raw<ChatItem[]>([]);
	let snapshotSynced = $state(false);

	// Live items while the store is bound to this session. Not a second copy.
	const liveItems = $derived(
		chatStore.sessionID === deps.getSessionId() ? chatStore.items : null
	);

	$effect(() => {
		const items = liveItems;
		if (!items) return;
		snapshotItems = items;
		snapshotSynced = true;
	});

	const hasVisibleConversation = $derived.by(() => {
		if (firstTurnActive || awaitingFirstAssistant) return true;
		const sessionId = deps.getSessionId();
		if (chatStore.sessionID === sessionId) return hasTurns(chatStore.items);
		// Status-only fork notes do not count, so EmptyChatState stays up.
		if (!chatStore.hasCachedConversationTurns(sessionId)) return false;
		if (!snapshotSynced) return true;
		return hasTurns(snapshotItems);
	});

	// One place applies dock/center. firstTurnActive returns early so a
	// mid-flight flag write cannot dock over the overlay.
	$effect(() => {
		const visible = hasVisibleConversation;
		const active = firstTurnActive;
		const awaiting = awaitingFirstAssistant;
		deps.syncComposerPhase({
			hasVisibleConversation: visible,
			firstTurnActive: active,
			awaitingFirstAssistant: awaiting
		});
		if (!visible && !active && !awaiting) {
			firstTurnFlightDone = false;
		}
	});

	function onSession(sessionId: string) {
		// Store reads stay untracked. Staging a user message mid-flight must
		// not re-enter this reset and clear firstTurnHandoffPending.
		untrack(() => {
			deps.abortFlights();
			firstTurnActive = false;
			firstTurnHandoffPending = false;
			const turns = chatStore.hasCachedConversationTurns(sessionId);
			awaitingFirstAssistant = chatStore.isAwaitingFirstAssistant(sessionId);
			// No user/assistant yet: explicitly false. Do NOT use !awaiting
			// (true when idle) which marks flight done after soft swaps.
			firstTurnFlightDone = turns;
			if (!turns && !awaitingFirstAssistant) {
				snapshotItems = [];
				snapshotSynced = true;
				shellStore.centerComposer();
			} else {
				snapshotSynced = false;
			}
			deps.syncQueueState();
		});
	}

	return {
		get hasVisibleConversation() {
			return hasVisibleConversation;
		},
		get firstTurnActive() {
			return firstTurnActive;
		},
		get firstTurnFlightDone() {
			return firstTurnFlightDone;
		},
		get firstTurnHandoffPending() {
			return firstTurnHandoffPending;
		},
		get awaitingFirstAssistant() {
			return awaitingFirstAssistant;
		},
		get snapshotSynced() {
			return snapshotSynced;
		},
		onSession,
		setFirstTurnActive(value: boolean) {
			firstTurnActive = value;
		},
		setFirstTurnFlightDone(value: boolean) {
			firstTurnFlightDone = value;
		},
		setFirstTurnHandoffPending(value: boolean) {
			firstTurnHandoffPending = value;
		},
		setAwaitingFirstAssistant(value: boolean) {
			awaitingFirstAssistant = value;
		}
	};
}
