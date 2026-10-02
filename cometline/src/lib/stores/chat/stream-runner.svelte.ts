import {
	abortSession,
	isSessionNotFoundError,
	streamMessage,
	streamSessionEvents
} from '#lib/client/cometmind.js';
import type { ChatItem } from '#lib/types.js';
import type { ChatTurnPayload } from '#lib/actions/start-chat.js';
import { messageContextRefsFromWebContexts } from '#lib/features/chat/message-context.js';
import { sessionStore } from '#lib/stores/session.svelte.js';
import { localID } from '#lib/stores/chat-transcript.js';
import type { SessionStream } from '#lib/stores/chat-stream-types.js';
import { isAbortError, isRunConflict, isSessionRunningConflict } from './chat-errors';
import type { AssistantItem } from './chat-turn';
import type { ChatState } from './chat-state.svelte';
import type { SessionCache } from './session-cache.svelte';
import type { SessionBinding } from './session-binding.svelte';
import type { TranscriptLoader } from './transcript-loader.svelte';
import type { RunOutcome, TurnEvents } from './turn-events.svelte';

export function createStreamRunner(
	state: ChatState,
	cache: SessionCache,
	binding: SessionBinding,
	loader: TranscriptLoader,
	turns: TurnEvents
) {
	const { applyEventToSession, ensureVisibleTurnFeedback, settleRunFeedback } = turns;

	async function send(
		nextSessionID: string,
		payloadOrText: ChatTurnPayload | string,
		opts?: { skipUser?: boolean; onConflict?: () => void }
	): Promise<void | 'session_running'> {
		const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
		const text = payload.text;
		const displayText = payload.displayText ?? text;
		const images = payload.images;
		if (cache.isStreamingFor(nextSessionID)) {
			throw new Error('Session is already streaming');
		}

		const handle: SessionStream = {
			run: ++state.globalStreamRun,
			abort: new AbortController(),
			ctx: {
				assistant: { current: null },
				reasoning: { current: null }
			}
		};

		state.sessionErrors.delete(nextSessionID);
		cache.setRunError(nextSessionID, false);
		if (state.sessionID === nextSessionID) {
			state.error = '';
		}

		const contexts = messageContextRefsFromWebContexts(payload.webContexts);
		cache.markStreaming(nextSessionID, handle);
		if (!opts?.skipUser)
			turns.addUserToSession(nextSessionID, displayText, images, true, contexts);

		const ctx = handle.ctx;
		const preId = localID('assistant');
		const preAssistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: preId,
			type: 'assistant',
			text: '',
			pending: true,
			pendingStartedAt: Date.now()
		};
		const preItems = cache.getCachedItems(nextSessionID).slice();
		preItems.push(preAssistant);
		cache.writeSessionItems(nextSessionID, preItems);
		ctx.assistant.current = preAssistant;
		let streamDone = false;
		let streamOutcome: RunOutcome = 'success';
		try {
			for await (const event of streamMessage(
				nextSessionID,
				{
					text,
					display_text: payload.displayText,
					images: images?.map((image) => ({
						media_type: image.media_type,
						data: image.data
					})),
					file_paths: payload.filePaths,
					web_contexts: payload.webContexts,
					reasoning_effort: payload.reasoningEffort,
					agent_mode: payload.agentMode
				},
				handle.abort.signal
			)) {
				const current = state.streamHandles.get(nextSessionID);
				if (!current || current.run !== handle.run) {
					handle.abort.abort();
					return;
				}
				if (event.type === 'done') {
					if (!streamDone) {
						streamDone = true;
						applyEventToSession(nextSessionID, event, ctx);
						cache.unmarkStreaming(nextSessionID);
						settleRunFeedback(nextSessionID, streamOutcome);
					}
					break;
				}
				if (event.type === 'error') streamOutcome = 'error';
				applyEventToSession(nextSessionID, event, ctx);
			}
		} catch (err) {
			const current = state.streamHandles.get(nextSessionID);
			if (!current || current.run !== handle.run) {
				handle.abort.abort();
				return;
			}
			if (isAbortError(err)) {
				streamOutcome = 'abort';
				return;
			}
			if (isSessionNotFoundError(err)) {
				streamOutcome = 'error';
				binding.discardMissingSession(nextSessionID);
				return;
			}
			if (isRunConflict(err)) {
				const sessionRunning = isSessionRunningConflict(err);
				if (!sessionRunning) opts?.onConflict?.();
				applyEventToSession(nextSessionID, { type: 'done' }, ctx);
				cache.unmarkStreaming(nextSessionID);
				await loader.refreshTranscript(nextSessionID);
				await resumeRun(nextSessionID);
				return sessionRunning ? 'session_running' : undefined;
			}
			streamOutcome = 'error';
			applyEventToSession(
				nextSessionID,
				{
					type: 'error',
					message: err instanceof Error ? err.message : 'Failed to send message'
				},
				ctx
			);
		} finally {
			if (state.streamHandles.get(nextSessionID) === handle) {
				if (!streamDone) {
					applyEventToSession(nextSessionID, { type: 'done' }, ctx);
					cache.unmarkStreaming(nextSessionID);
					settleRunFeedback(nextSessionID, streamOutcome);
				}
				// Mini models / aborted streams can settle with no visible assistant
				// content. Never leave a blank first-turn avatar with no feedback.
				ensureVisibleTurnFeedback(nextSessionID, ctx, streamOutcome);
			}
		}
	}

	async function resumeRun(targetSessionID: string) {
		if (state.streamHandles.has(targetSessionID)) return;

		const handle: SessionStream = {
			run: ++state.globalStreamRun,
			abort: new AbortController(),
			ctx: {
				assistant: { current: null },
				reasoning: { current: null }
			}
		};
		const cached = cache.getCachedItems(targetSessionID);
		const lastUserIndex = cached.findLastIndex((item) => item.type === 'user');
		const currentTurnAssistant = cached.findLast(
			(item, index): item is AssistantItem =>
				item.type === 'assistant' && index > lastUserIndex
		);
		if (currentTurnAssistant) {
			handle.ctx.assistant.current = currentTurnAssistant;
		} else {
			const assistant: AssistantItem = {
				id: localID('assistant'),
				type: 'assistant',
				text: '',
				pending: true,
				pendingStartedAt: Date.now()
			};
			cache.writeSessionItems(targetSessionID, [...cached, assistant]);
			handle.ctx.assistant.current = assistant;
		}

		state.sessionErrors.delete(targetSessionID);
		cache.setRunError(targetSessionID, false);
		cache.markStreaming(targetSessionID, handle);
		sessionStore.setRunning(targetSessionID, true);
		let streamDone = false;
		let streamOutcome: RunOutcome = 'success';
		let refreshAfterConflict = false;
		try {
			for await (const event of streamSessionEvents(targetSessionID, handle.abort.signal)) {
				const current = state.streamHandles.get(targetSessionID);
				if (!current || current.run !== handle.run) {
					handle.abort.abort();
					return;
				}
				if (event.type === 'done') {
					streamDone = true;
					applyEventToSession(targetSessionID, event, handle.ctx);
					cache.unmarkStreaming(targetSessionID);
					sessionStore.setRunning(targetSessionID, false);
					settleRunFeedback(targetSessionID, streamOutcome);
					break;
				}
				if (event.type === 'error') streamOutcome = 'error';
				applyEventToSession(targetSessionID, event, handle.ctx);
			}
		} catch (err) {
			const current = state.streamHandles.get(targetSessionID);
			if (!current || current.run !== handle.run) return;
			if (isAbortError(err)) {
				streamOutcome = 'abort';
				return;
			}
			if (isRunConflict(err)) {
				refreshAfterConflict = true;
				return;
			}
			if (isSessionNotFoundError(err)) {
				binding.discardMissingSession(targetSessionID);
				return;
			}
			streamOutcome = 'error';
			applyEventToSession(
				targetSessionID,
				{
					type: 'error',
					message: err instanceof Error ? err.message : 'Failed to resume response'
				},
				handle.ctx
			);
		} finally {
			if (state.streamHandles.get(targetSessionID) === handle) {
				if (!streamDone) {
					applyEventToSession(targetSessionID, { type: 'done' }, handle.ctx);
					cache.unmarkStreaming(targetSessionID);
					sessionStore.setRunning(targetSessionID, false);
					if (!refreshAfterConflict) {
						settleRunFeedback(targetSessionID, streamOutcome);
						ensureVisibleTurnFeedback(targetSessionID, handle.ctx, streamOutcome);
					}
				}
			}
			if (refreshAfterConflict) await loader.refreshTranscript(targetSessionID);
		}
	}

	async function cancel(targetSessionID?: string) {
		const id = targetSessionID ?? state.sessionID;
		if (!id) return;
		const handle = state.streamHandles.get(id);
		const running =
			sessionStore.sessions.find((session) => session.id === id)?.running ?? false;
		if (!handle && !cache.isStreamingFor(id) && !running) return;

		handle?.abort.abort();
		try {
			await abortSession(id);
		} catch {
			return;
		}
	}

	async function cancelSubagent(childSessionId: string) {
		if (!state.sessionID) return;
		try {
			await abortSession(childSessionId);
			turns.patchSubagentCard(state.sessionID, childSessionId, {
				status: 'cancelled',
				pending: false
			});
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Failed to cancel subagent';
		}
	}

	return { send, resumeRun, cancel, cancelSubagent };
}
