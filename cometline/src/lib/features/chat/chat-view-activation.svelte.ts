import { tick } from 'svelte';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { composerHistoryStore } from '#lib/stores/composer-history.svelte.js';
import { shouldApplyComposerFocus } from '#lib/features/chat/composer-focus.js';
import type { ChatTurnPayload } from '#lib/actions/start-chat.js';
import type { PendingUnsentDraft } from '#lib/features/composer/composer-history.js';

export interface ChatViewComposerHandle {
	focus: () => void;
	restoreDraft: (draft: PendingUnsentDraft) => boolean;
}

export function createChatViewActivation(deps: {
	getSessionId: () => string;
	getComposer: () => ChatViewComposerHandle | null;
	bindSession: () => void;
	syncSessionFromStore: () => void;
	onMount: () => void;
}) {
	const composerFocusRequest = $derived(shellStore.composerFocusRequest);
	let lastAppliedComposerFocusId = $state(0);
	let activatedSessionId = $state<string | null>(null);
	let activationRun = 0;

	function restoreRejectedTurn(rejectedSessionId: string, payload: ChatTurnPayload) {
		const draft: PendingUnsentDraft = {
			text: payload.displayText ?? payload.text,
			images: payload.images
		};
		composerHistoryStore.stashUnsent(rejectedSessionId, draft);
		if (rejectedSessionId === deps.getSessionId()) deps.getComposer()?.restoreDraft(draft);
	}

	async function activateSession(id: string, run: number) {
		await tick();
		if (activationRun !== run || deps.getSessionId() !== id) return;
		const pendingDraft = composerHistoryStore.getPending(id);
		if (pendingDraft) deps.getComposer()?.restoreDraft(pendingDraft);
		deps.onMount();
		if (shellStore.focusedPane !== 'chat') return;
		if (composerFocusRequest.sessionId !== id) {
			shellStore.requestComposerFocus(id);
		}
		// The request effect can run before this session finishes binding. Retry
		// after activation so a main-window route switch always reaches its composer.
		deps.getComposer()?.focus();
	}

	/** Effect body: binds and activates each newly shown session once. */
	function activate() {
		const sessionId = deps.getSessionId();
		if (!sessionId) return;
		if (activatedSessionId === sessionId) return;
		activatedSessionId = sessionId;
		const run = ++activationRun;
		deps.bindSession();
		deps.syncSessionFromStore();
		void activateSession(sessionId, run);
	}

	/** Effect body: applies shell composer-focus requests addressed to this session. */
	function applyComposerFocusRequest() {
		if (
			!shouldApplyComposerFocus({
				requestId: composerFocusRequest.id,
				requestSessionId: composerFocusRequest.sessionId,
				sessionId: deps.getSessionId(),
				focusedPane: shellStore.focusedPane,
				lastAppliedRequestId: lastAppliedComposerFocusId
			})
		)
			return;
		lastAppliedComposerFocusId = composerFocusRequest.id;
		deps.getComposer()?.focus();
	}

	return { restoreRejectedTurn, activate, applyComposerFocusRequest };
}
