/**
 * ConversationController — single module for chat turn orchestration.
 *
 * Owns turn queue serialization, start-chat decision tree (flight / skipUser),
 * pending-first-message consumption, and transcript load gating. ChatView stays
 * presentation-only and wires flight components through adapters.
 */

import { getSession } from '$lib/client/cometmind';
import { commitSidebarWorkspaceForSession } from '$lib/actions/commit-sidebar-workspace';
import {
	createChatTurnQueue,
	type ChatTurnQueue,
	type QueuedMessage
} from '$lib/actions/chat-turn-queue';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import type { ImageAttachment } from '$lib/types';
import type { ChatTurnPayload } from '$lib/actions/start-chat';

export type { ChatTurnPayload } from '$lib/actions/start-chat';
export type { QueuedMessage } from '$lib/actions/chat-turn-queue';

export interface ConversationFlightAdapter {
	onUserMessageFlight(
		payload: ChatTurnPayload | string,
		ctx: { firstTurn: boolean }
	): void | Promise<void>;
	onFirstTurnComplete?(): void;
}

export interface ConversationControllerDeps {
	getSessionId: () => string;
	getHasVisibleConversation: () => boolean;
	send: (
		sessionId: string,
		payload: ChatTurnPayload | string,
		opts?: { skipUser?: boolean }
	) => Promise<void>;
	refreshSession: (sessionId: string) => Promise<void>;
	flight?: ConversationFlightAdapter;
	onQueueChange?: () => void;
	onAwaitingFirstAssistantChange?: (value: boolean) => void;
}

export interface ConversationController {
	get sessionId(): string;
	readonly pendingCount: number;
	readonly pendingMessages: readonly QueuedMessage[];
	readonly processing: boolean;
	bindSession(): void;
	shouldSkipTranscriptLoad(): boolean;
	onMount(): void;
	syncComposerPhase(opts: {
		hasVisibleConversation: boolean;
		firstTurnActive: boolean;
		awaitingFirstAssistant: boolean;
	}): void;
	enqueue(text: string, images?: ImageAttachment[], filePaths?: string[]): Promise<boolean>;
	removeQueued(id: string): boolean;
	cancel(): void;
}

async function runTurn(
	deps: ConversationControllerDeps,
	turnSessionId: string,
	payloadOrText: ChatTurnPayload | string,
	getHasVisibleConversation: () => boolean
): Promise<void> {
	const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
	const firstTurn = !getHasVisibleConversation();
	const usesFlight = Boolean(deps.flight?.onUserMessageFlight);

	commitSidebarWorkspaceForSession(
		sessionStore.sessions.find((session) => session.id === turnSessionId) ??
			sessionStore.current
	);

	if (usesFlight) {
		await deps.flight!.onUserMessageFlight!(payload.images?.length ? payload : payload.text, {
			firstTurn
		});
	}

	await deps.send(turnSessionId, payloadOrText, { skipUser: usesFlight ? true : firstTurn });

	if (firstTurn) {
		deps.onAwaitingFirstAssistantChange?.(false);
		deps.flight?.onFirstTurnComplete?.();
	}

	void deps.refreshSession(turnSessionId);
}

export function createConversationController(
	deps: ConversationControllerDeps
): ConversationController {
	let turnQueue: ChatTurnQueue | undefined;
	let queueSessionId: string | null = null;

	function ensureQueue(): ChatTurnQueue {
		const sessionId = deps.getSessionId();
		if (!turnQueue || queueSessionId !== sessionId) {
			const queueForSessionId = sessionId;
			queueSessionId = sessionId;
			turnQueue = createChatTurnQueue(async (text, images, filePaths) => {
				if (images === undefined && filePaths === undefined) {
					await runTurn(deps, queueForSessionId, text, deps.getHasVisibleConversation);
				} else if (filePaths === undefined) {
					await runTurn(
						deps,
						queueForSessionId,
						{ text, images },
						deps.getHasVisibleConversation
					);
				} else {
					await runTurn(
						deps,
						queueForSessionId,
						{ text, images, filePaths },
						deps.getHasVisibleConversation
					);
				}
			}, deps.onQueueChange);
		}
		return turnQueue;
	}

	return {
		get sessionId() {
			return deps.getSessionId();
		},

		get pendingCount() {
			return ensureQueue().pendingCount;
		},
		get pendingMessages() {
			return ensureQueue().pendingMessages;
		},
		get processing() {
			return ensureQueue().processing;
		},

		bindSession() {
			chatStore.bindSession(deps.getSessionId());
			if (shellStore.composerPhase === 'docked' || chatStore.isLoading) {
				shellStore.dockComposer();
			}
		},

		shouldSkipTranscriptLoad() {
			const sessionId = deps.getSessionId();
			if (sessionStore.hasPendingMessage(sessionId)) return true;
			if (chatStore.hasInFlightTurn(sessionId)) return true;
			if (chatStore.sessionID === sessionId && chatStore.items.length > 0) return true;
			return false;
		},

		onMount() {
			const sessionId = deps.getSessionId();
			const pending = sessionStore.takePendingMessage(sessionId);
			if (pending) {
				void ensureQueue().enqueue(pending.text, pending.images, pending.filePaths);
				return;
			}
			if (!this.shouldSkipTranscriptLoad()) {
				void chatStore.loadTranscript(sessionId);
			}
		},

		syncComposerPhase(opts) {
			const { hasVisibleConversation, firstTurnActive, awaitingFirstAssistant } = opts;
			if (chatStore.sessionID !== deps.getSessionId()) return;
			if (firstTurnActive) return;

			if (hasVisibleConversation) {
				shellStore.dockComposer();
			} else if (!chatStore.isLoading && !awaitingFirstAssistant) {
				shellStore.centerComposer();
			}
		},

		enqueue(text: string, images?: ImageAttachment[], filePaths?: string[]) {
			return ensureQueue().enqueue(text, images, filePaths);
		},

		removeQueued(id: string) {
			return ensureQueue().remove(id);
		},

		cancel() {
			void chatStore.cancel(deps.getSessionId());
		}
	};
}

/** Refresh session metadata after a turn (title, etc.). */
export async function refreshConversationSession(sessionId: string): Promise<void> {
	try {
		sessionStore.updateSession(await getSession(sessionId));
	} catch {
		// Transcript is source of truth; title refresh is best effort.
	}
}
