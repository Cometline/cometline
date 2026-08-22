/**
 * ConversationController — single module for chat turn orchestration.
 *
 * Owns turn queue serialization, start-chat decision tree (flight / skipUser),
 * pending-first-message consumption, and transcript load gating. ChatView stays
 * presentation-only and wires flight components through adapters.
 */

import { tick } from 'svelte';
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
import { messageContextRefsFromWebContexts } from '$lib/chat/message-context';

export type { ChatTurnPayload } from '$lib/actions/start-chat';
export type { QueuedMessage } from '$lib/actions/chat-turn-queue';

export interface ConversationFlightAdapter {
	onUserMessageFlight(
		payload: ChatTurnPayload | string,
		ctx: {
			firstTurn: boolean;
			sessionId: string;
			stageUser: (text: string, images?: ImageAttachment[]) => string;
			revealStagedUser: () => void;
			userItemId?: string;
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
		opts?: { skipUser?: boolean; onConflict?: () => void }
	) => Promise<void>;
	refreshSession: (sessionId: string) => Promise<void>;
	flight?: ConversationFlightAdapter;
	onQueueChange?: () => void;
	onAwaitingFirstAssistantChange?: (value: boolean) => void;
	onTurnRejected?: (sessionId: string, payload: ChatTurnPayload) => void;
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
	const firstTurn =
		usesFlight && !isViewing
			? chatStore.getCachedItemCount(turnSessionId) === 0
			: !getHasVisibleConversation();
	const flightPayload = payload.images?.length ? payload : userDisplay;
	const contexts = messageContextRefsFromWebContexts(payload.webContexts);
	let stagedUserId: string | undefined;
	let flightPromise: Promise<void> | undefined;
	let sendPromise: Promise<void> | undefined;
	const startSend = () => {
		if (!sendPromise) {
			const opts: { skipUser?: boolean; onConflict?: () => void } = {
				skipUser: usesFlight ? true : firstTurn
			};
			if (deps.onTurnRejected) {
				opts.onConflict = () => deps.onTurnRejected?.(turnSessionId, payload);
			}
			sendPromise = deps.send(turnSessionId, payloadOrText, opts);
			void sendPromise.catch(() => undefined);
		}
		return sendPromise;
	};
	const stageUser = (text: string, images?: ImageAttachment[]) => {
		stagedUserId ??= chatStore.stageUserForSession(turnSessionId, text, images, contexts);
		void startSend();
		return stagedUserId;
	};
	const revealStagedUser = () => {
		chatStore.revealStagedUserForSession(turnSessionId);
	};

	commitSidebarWorkspaceForSession(
		sessionStore.sessions.find((session) => session.id === turnSessionId) ??
			sessionStore.current
	);

	if (usesFlight && isViewing && firstTurn) {
		try {
			await deps.flight!.onUserMessageFlight!(flightPayload, {
				firstTurn,
				sessionId: turnSessionId,
				stageUser,
				revealStagedUser
			});
		} catch {
			// A flight failure must not lose or strand the user turn.
			if (!stagedUserId) stageUser(userDisplay, payload.images);
		} finally {
			revealStagedUser();
		}
	} else if (usesFlight && isViewing) {
		// Follow-up: no particle flight — stage hidden, then short-fade reveal.
		// Pin scroll owns layout; first-turn keeps the flight choreography above.
		stageUser(userDisplay, payload.images);
		flightPromise = tick().then(() => {
			revealStagedUser();
		});
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
	} else {
		// A session queue can outlive its ChatView while it drains in the background.
		// Point change notifications at the view that is currently showing that session.
		queue.setOnChange(deps.onQueueChange);
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
			const queue = ensureQueue(sessionId, deps, deps.getHasVisibleConversation);
			const pending = sessionStore.takePendingMessage(sessionId);
			if (pending) {
				void queue.enqueue({
					text: pending.text,
					displayText: pending.displayText,
					images: pending.images,
					filePaths: pending.filePaths,
					webContexts: pending.webContexts,
					agentMode: pending.agentMode
				});
				return;
			}
			const activation = (async () => {
				let running =
					sessionStore.sessions.find((session) => session.id === sessionId)?.running ??
					false;
				try {
					const latest = await getSession(sessionId);
					running = latest.running;
					sessionStore.updateSession(latest);
				} catch {
					// Transcript loading retains the existing not-found and retry behavior.
				}
				if (!this.shouldSkipTranscriptLoad()) {
					await chatStore.loadTranscript(sessionId);
				}
				if (running) await chatStore.resumeRun(sessionId);
			})();
			queue.blockUntil(activation);
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
	const apply = async () => {
		const latest = await getSession(sessionId);
		sessionStore.patchSessionMetadata(sessionId, {
			title: latest.title,
			token_usage: latest.token_usage,
			updated_at: latest.updated_at
		});
	};
	try {
		await apply();
		// Title LLM runs asynchronously after the first message; retry once so
		// the sidebar can pick up the generated title without another turn.
		window.setTimeout(() => {
			void apply().catch(() => undefined);
		}, 2500);
	} catch {
		// Transcript is source of truth; title refresh is best effort.
	}
}

/** @internal Test helper — reset module-level turn queues between tests. */
export function resetConversationTurnQueuesForTests() {
	turnQueues.clear();
}
