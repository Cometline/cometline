import type { ChatItem, ImageAttachment, MessageContextRef, StreamEvent } from '#lib/types.js';
import { reduceChatState } from '#lib/reducers/chat.js';
import { playErrorSound, playResponseCompleteSound } from '#lib/sound/response-complete.js';
import { settingsStore } from '#lib/stores/settings.svelte.js';
import { unreadSessionOutputStore } from '#lib/stores/unread-session-output.svelte.js';
import { localID } from '#lib/stores/chat-transcript.js';
import type { StreamCtx } from '#lib/stores/chat-stream-types.js';
import { reconcileStreamCtx, turnHasVisibleContent } from './chat-turn';
import type { ChatState } from './chat-state.svelte';
import type { SessionCache } from './session-cache.svelte';

export type RunOutcome = 'success' | 'abort' | 'error';

export function createTurnEvents(state: ChatState, cache: SessionCache) {
	function addUserToSession(
		targetSessionID: string,
		text: string,
		images?: ImageAttachment[],
		reveal = true,
		contexts?: MessageContextRef[]
	) {
		const next = cache.getCachedItems(targetSessionID).slice();
		const id = localID('user');
		next.push({
			id,
			type: 'user',
			text,
			images,
			...(contexts?.length ? { contexts } : {}),
			reveal
		});
		cache.writeSessionItems(targetSessionID, next);
		return id;
	}

	function stageUserForSession(
		targetSessionID: string,
		text: string,
		images?: ImageAttachment[],
		contexts?: MessageContextRef[]
	) {
		return addUserToSession(targetSessionID, text, images, false, contexts);
	}

	function revealStagedUserForSession(targetSessionID: string) {
		const current = cache.getCachedItems(targetSessionID);
		let revealIndex = -1;
		for (let i = current.length - 1; i >= 0; i--) {
			const item = current[i];
			if (item.type === 'user' && item.reveal === false) {
				revealIndex = i;
				break;
			}
		}
		if (revealIndex < 0) return;
		cache.writeSessionItems(
			targetSessionID,
			current.map((item, i) =>
				i === revealIndex && item.type === 'user' ? { ...item, reveal: true } : item
			)
		);
	}

	function applyEventToSession(targetSessionID: string, event: StreamEvent, ctx: StreamCtx) {
		if (cache.isStreamingFor(targetSessionID)) {
			reconcileStreamCtx(cache.getCachedItems(targetSessionID), ctx);
		}
		const sessionItems = cache.getCachedItems(targetSessionID);
		const sessionError =
			state.sessionErrors.get(targetSessionID) ??
			(state.sessionID === targetSessionID ? state.error : '');
		const reduced = reduceChatState(
			{
				items: sessionItems,
				error: sessionError,
				assistant: ctx.assistant.current,
				reasoning: ctx.reasoning.current,
				nextId: state.nextId,
				contextBudget: state.sessionContextBudgets.get(targetSessionID) ?? null
			},
			event
		);
		state.nextId = reduced.nextId;
		ctx.assistant.current = reduced.assistant;
		ctx.reasoning.current = reduced.reasoning;
		if (reduced.contextBudget) {
			state.sessionContextBudgets.set(targetSessionID, reduced.contextBudget);
		} else {
			state.sessionContextBudgets.delete(targetSessionID);
		}
		if (reduced.error) {
			state.sessionErrors.set(targetSessionID, reduced.error);
		} else {
			state.sessionErrors.delete(targetSessionID);
		}
		if (state.sessionID === targetSessionID) {
			state.error = reduced.error;
			state.contextBudget = reduced.contextBudget;
		}
		cache.writeSessionItems(targetSessionID, reduced.items);
		if (event.type !== 'done' && state.sessionID !== targetSessionID) {
			unreadSessionOutputStore.markUnread(targetSessionID);
		}
	}

	/**
	 * After stream settle, if nothing visible remains (common after cancel mid-stream
	 * or mini-model empty finish), surface a clear in-thread error instead of a blank avatar.
	 */
	function ensureVisibleTurnFeedback(
		targetSessionID: string,
		ctx: StreamCtx,
		outcome: RunOutcome
	) {
		// A user-initiated stop is a successful control action. The done reducer
		// has already settled or removed the pending assistant placeholder.
		if (outcome === 'abort') return;
		const sessionItems = cache.getCachedItems(targetSessionID);
		if (turnHasVisibleContent(sessionItems, ctx.assistant.current)) {
			return;
		}
		if (state.sessionErrors.get(targetSessionID)?.trim()) {
			return;
		}
		const message =
			outcome === 'error'
				? 'The request failed before a reply was ready. Try again.'
				: 'The model finished without a visible reply. Try again, or switch to a stronger model.';
		applyEventToSession(
			targetSessionID,
			{
				type: 'error',
				message
			},
			ctx
		);
	}

	function settleRunFeedback(targetSessionID: string, outcome: RunOutcome) {
		const failed = outcome === 'error' || Boolean(state.sessionErrors.get(targetSessionID));
		cache.setRunError(targetSessionID, failed);
		const soundSettings = settingsStore.settings.appearance.responseCompleteSound;
		if (failed) {
			playErrorSound(soundSettings);
		} else {
			playResponseCompleteSound(soundSettings);
		}
	}

	function patchSubagentCard(
		targetSessionID: string,
		childSessionId: string,
		patch: Partial<Extract<ChatItem, { type: 'subagent' }>>
	) {
		const next = cache
			.getCachedItems(targetSessionID)
			.map((item) =>
				item.type === 'subagent' && item.childSessionId === childSessionId
					? { ...item, ...patch }
					: item
			);
		cache.writeSessionItems(targetSessionID, next);
	}

	return {
		addUserToSession,
		stageUserForSession,
		revealStagedUserForSession,
		applyEventToSession,
		ensureVisibleTurnFeedback,
		settleRunFeedback,
		patchSubagentCard
	};
}

export type TurnEvents = ReturnType<typeof createTurnEvents>;
