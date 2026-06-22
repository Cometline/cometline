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
		ctx: {
			firstTurn: boolean;
			sessionId: string;
			stageUser: (text: string, images?: ImageAttachment[]) => void;
			revealStagedUser: () => void;
		}
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
	enqueue(payload: ChatTurnPayload | string): Promise<boolean>;
	removeQueued(id: string): boolean;
	clearQueue(): void;
	cancel(): void;
}

const turnQueues = new Map<string, ChatTurnQueue>();

async function runTurn(
	deps: ConversationControllerDeps,
	turnSessionId: string,
	payloadOrText: ChatTurnPayload | string,
	getHasVisibleConversation: () => boolean
): Promise<void> {
	const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
	const userDisplay = payload.displayText ?? payload.text;
	const usesFlight = Boolean(deps.flight?.onUserMessageFlight);
	const isViewing = deps.getSessionId() === turnSessionId;
	const firstTurn = usesFlight && !isViewing
		? chatStore.getCachedItemCount(turnSessionId) === 0
		: !getHasVisibleConversation();
	const flightPayload = payload.images?.length ? payload : userDisplay;
	const stageUser = (text: string, images?: ImageAttachment[]) => {
		chatStore.stageUserForSession(turnSessionId, text, images);
	};
	const revealStagedUser = () => {
		chatStore.revealStagedUserForSession(turnSessionId);
	};
	let flightPromise: Promise<void> | undefined;
	let sendPromise: Promise<void> | undefined;
	const startSend = () => {
		if (!sendPromise) {
			sendPromise = deps.send(turnSessionId, payloadOrText, {
				skipUser: usesFlight ? true : firstTurn
			});
			void sendPromise.catch(() => undefined);
		}
		return sendPromise;
	};

	commitSidebarWorkspaceForSession(
		sessionStore.sessions.find((session) => session.id === turnSessionId) ??
			sessionStore.current
	);

	if (usesFlight && isViewing && firstTurn) {
		await deps.flight!.onUserMessageFlight!(flightPayload, {
			firstTurn,
			sessionId: turnSessionId,
			stageUser: (text, images) => {
				stageUser(text, images);
				void startSend();
			},
			revealStagedUser
		});
	} else if (usesFlight && isViewing) {
		stageUser(userDisplay, payload.images);
		flightPromise = Promise.resolve(
			deps.flight!.onUserMessageFlight!(flightPayload, {
				firstTurn,
				sessionId: turnSessionId,
				stageUser,
				revealStagedUser
			})
		)
			.catch(() => undefined)
			.finally(revealStagedUser);
	} else if (usesFlight) {
		stageUser(userDisplay, payload.images);
		revealStagedUser();
	}

	await startSend();
	if (flightPromise) await flightPromise;

	if (firstTurn) {
		deps.onAwaitingFirstAssistantChange?.(false);
		deps.flight?.onFirstTurnComplete?.();
	}

	void deps.refreshSession(turnSessionId);
}

function ensureQueue(
	sessionId: string,
	deps: ConversationControllerDeps,
	getHasVisibleConversation: () => boolean
): ChatTurnQueue {
	let queue = turnQueues.get(sessionId);
	if (!queue) {
		const queueForSessionId = sessionId;
		queue = createChatTurnQueue(async (payload) => {
			await runTurn(deps, queueForSessionId, payload, getHasVisibleConversation);
		}, deps.onQueueChange);
		turnQueues.set(sessionId, queue);
	}
	return queue;
}

export function createConversationController(
	deps: ConversationControllerDeps
): ConversationController {
	function queueForCurrentSession(): ChatTurnQueue {
		return ensureQueue(deps.getSessionId(), deps, deps.getHasVisibleConversation);
	}

	return {
		get sessionId() {
			return deps.getSessionId();
		},

		get pendingCount() {
			return queueForCurrentSession().pendingCount;
		},
		get pendingMessages() {
			return queueForCurrentSession().pendingMessages;
		},
		get processing() {
			return queueForCurrentSession().processing;
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
			if (chatStore.getCachedItemCount(sessionId) > 0) return true;
			return false;
		},

		onMount() {
			const sessionId = deps.getSessionId();
			const pending = sessionStore.takePendingMessage(sessionId);
			if (pending) {
				void ensureQueue(sessionId, deps, deps.getHasVisibleConversation).enqueue({
					text: pending.text,
					displayText: pending.displayText,
					images: pending.images,
					filePaths: pending.filePaths
				});
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

	enqueue(payload: ChatTurnPayload | string) {
			return ensureQueue(deps.getSessionId(), deps, deps.getHasVisibleConversation).enqueue(
				payload
			);
		},

		removeQueued(id: string) {
			return queueForCurrentSession().remove(id);
		},

		clearQueue() {
			queueForCurrentSession().clear();
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

/** @internal Test helper — reset module-level turn queues between tests. */
export function resetConversationTurnQueuesForTests() {
	turnQueues.clear();
}
