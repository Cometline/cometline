/**
 * ConversationController — single module for chat turn orchestration.
 *
 * Owns turn queue serialization, start-chat decision tree (flight / skipUser),
 * pending-first-message consumption, and transcript load gating. ChatView stays
 * presentation-only and wires flight components through adapters.
 */

import { tick } from 'svelte';
import { CometMindApiError, compactSessionContext, getSession } from '#lib/client/cometmind.js';
import { commitSidebarWorkspaceForSession } from '#lib/actions/commit-sidebar-workspace.js';
import {
	createChatTurnQueue,
	type ChatTurnQueue,
	type QueuedMessage
} from '#lib/actions/chat-turn-queue.js';
import { chatStore } from '#lib/stores/chat.svelte.js';
import { sessionStore } from '#lib/stores/session.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import type { ImageAttachment } from '#lib/types.js';
import type { ChatTurnPayload } from '#lib/actions/start-chat.js';
import { messageContextRefsFromWebContexts } from '#lib/features/chat/message-context.js';
import { compactionFeedback } from '#lib/features/chat/context-compaction.js';
import { contextCompactionStore } from '#lib/features/chat/context-compaction.svelte.js';
import { appToastStore } from '#lib/stores/app-toasts.svelte.js';

export type { ChatTurnPayload } from '#lib/actions/start-chat.js';
export type { QueuedMessage } from '#lib/actions/chat-turn-queue.js';

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
	send: (
		sessionId: string,
		payload: ChatTurnPayload | string,
		opts?: { skipUser?: boolean; onConflict?: () => void }
	) => Promise<void | 'session_running' | 'session_compacting'>;
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
	compact(): void;
}

const turnQueues = new Map<string, ChatTurnQueue>();

async function runTurn(
	deps: ConversationControllerDeps,
	turnSessionId: string,
	payloadOrText: ChatTurnPayload | string
): Promise<void> {
	const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
	const userDisplay = payload.displayText ?? payload.text;
	const usesFlight = Boolean(deps.flight?.onUserMessageFlight);
	const isViewing = deps.getSessionId() === turnSessionId;
	// No user/assistant yet — not hasVisibleConversation() / raw item count.
	// Fork AppendSystemMessage becomes a status row and must not skip FirstTurnFlight.
	const firstTurn = !chatStore.hasCachedConversationTurns(turnSessionId);
	const flightPayload = payload.images?.length ? payload : userDisplay;
	const contexts = messageContextRefsFromWebContexts(payload.webContexts);
	let stagedUserId: string | undefined;
	let flightPromise: Promise<void> | undefined;
	let sendPromise: Promise<void> | undefined;
	let sendAttempts = 0;
	const startSend = () => {
		if (!sendPromise) {
			sendPromise = (async () => {
				// session_running rejects before persistence and refreshes the transcript,
				// which drops the optimistic user row, so that retry must restage it.
				// session_compacting does not refresh, so the shown row must stay.
				let keepShownUser = false;
				while (true) {
					const skipUser =
						keepShownUser || (sendAttempts === 0 && (usesFlight || firstTurn));
					const opts: { skipUser?: boolean; onConflict?: () => void } = {
						skipUser
					};
					if (deps.onTurnRejected) {
						opts.onConflict = () => deps.onTurnRejected?.(turnSessionId, payload);
					}
					sendAttempts += 1;
					const outcome = await deps.send(turnSessionId, payloadOrText, opts);
					if (outcome === 'session_compacting') {
						keepShownUser = true;
						await waitForContextCompaction(turnSessionId);
						continue;
					}
					keepShownUser = false;
					if (outcome !== 'session_running') return;
				}
			})();
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

function ensureQueue(sessionId: string, deps: ConversationControllerDeps): ChatTurnQueue {
	let queue = turnQueues.get(sessionId);
	if (!queue) {
		const queueForSessionId = sessionId;
		queue = createChatTurnQueue(async (payload) => {
			await runTurn(deps, queueForSessionId, payload);
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
		return ensureQueue(deps.getSessionId(), deps);
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
			const sessionId = deps.getSessionId();
			chatStore.bindSession(sessionId);
			// Remount-equivalent: empty sessions center (hero), sessions with
			// content/in-flight dock. Do not dock solely because isLoading — that
			// leaves the composer stuck docked after /change → empty fork.
			if (
				chatStore.hasCachedConversationTurns(sessionId) ||
				chatStore.hasInFlightTurn(sessionId)
			) {
				shellStore.dockComposer();
			} else {
				shellStore.centerComposer();
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
			const queue = ensureQueue(sessionId, deps);
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
				let compacting = contextCompactionStore.isCompacting(sessionId);
				try {
					const latest = await getSession(sessionId);
					running = latest.running;
					compacting = compacting || Boolean(latest.compacting);
					sessionStore.updateSession(latest);
				} catch {
					// Transcript loading retains the existing not-found and retry behavior.
				}
				if (!this.shouldSkipTranscriptLoad()) {
					await chatStore.loadTranscript(sessionId);
				}
				if (running && !compacting) {
					await chatStore.resumeRun(sessionId);
				}
			})();
			queue.blockUntil(activation);
		},

		syncComposerPhase(opts) {
			const { hasVisibleConversation, firstTurnActive, awaitingFirstAssistant } = opts;
			if (chatStore.sessionID !== deps.getSessionId()) return;
			if (firstTurnActive) return;

			const sessionId = deps.getSessionId();
			const empty = !chatStore.hasCachedConversationTurns(sessionId);
			// Soft /change into an empty fork (status-only system note counts as empty):
			// mid-switch visibility flags must not dock after we just centered —
			// emptiness wins until real user/assistant content or first-turn prepare.
			if (empty && !awaitingFirstAssistant) {
				shellStore.centerComposer();
				return;
			}

			if (hasVisibleConversation) {
				shellStore.dockComposer();
			} else if (!awaitingFirstAssistant) {
				// Keep hero/EmptyChatState while an empty transcript loads — do not
				// wait for !isLoading (that used to leave a docked composer stuck).
				shellStore.centerComposer();
			}
		},

		enqueue(payload: ChatTurnPayload | string) {
			return ensureQueue(deps.getSessionId(), deps).enqueue(payload);
		},

		removeQueued(id: string) {
			return queueForCurrentSession().remove(id);
		},

		clearQueue() {
			queueForCurrentSession().clear();
		},

		cancel() {
			void chatStore.cancel(deps.getSessionId());
		},

		compact() {
			const sessionId = deps.getSessionId();
			if (!sessionId || contextCompactionStore.isCompacting(sessionId)) return;
			if (chatStore.isStreamingFor(sessionId) || chatStore.hasInFlightTurn(sessionId)) return;
			const current = sessionStore.sessions.find((session) => session.id === sessionId);
			if (current?.running && !current.compacting) return;
			const queue = ensureQueue(sessionId, deps);
			if (queue.processing) return;

			contextCompactionStore.begin(sessionId);
			if (current) {
				sessionStore.updateSession({ ...current, running: true, compacting: true });
			} else {
				sessionStore.setRunning(sessionId, true);
			}
			queue.blockUntil(runContextCompaction(sessionId));
		}
	};
}

async function runContextCompaction(sessionId: string): Promise<void> {
	try {
		const result = await compactSessionContext(sessionId);
		const feedback = compactionFeedback(result);
		if (feedback?.tone === 'success') appToastStore.success(feedback.label, feedback.detail);
		if (feedback?.tone === 'warning') appToastStore.warning(feedback.label, feedback.detail);
		if (feedback?.tone === 'error') appToastStore.error(feedback.label, feedback.detail);
		if (result.status === 'compacted' && result.context_window) {
			chatStore.noteContextBudget(sessionId, {
				estimated: result.estimated ?? 0,
				available: result.available ?? 0,
				contextWindow: result.context_window,
				compacted: result.status === 'compacted'
			});
		}
	} catch (err) {
		if (err instanceof CometMindApiError && err.status === 409) return;
		const detail = err instanceof Error ? err.message : 'Context compaction failed';
		appToastStore.error('Could not compact context', detail);
	} finally {
		contextCompactionStore.end(sessionId);
		try {
			const latest = await getSession(sessionId);
			sessionStore.updateSession(latest);
		} catch {
			const existing = sessionStore.sessions.find((session) => session.id === sessionId);
			if (existing) {
				sessionStore.updateSession({ ...existing, running: false, compacting: false });
			}
		}
	}
}

async function waitForContextCompaction(sessionId: string): Promise<void> {
	const deadline = Date.now() + 5 * 60_000;
	while (Date.now() < deadline) {
		try {
			const latest = await getSession(sessionId);
			sessionStore.updateSession(latest);
			if (!latest.compacting) return;
		} catch {
			return;
		}
		await new Promise((resolve) => setTimeout(resolve, 500));
	}
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
