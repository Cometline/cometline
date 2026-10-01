import { connectionState } from '$lib/stores/runtime.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { modelStore, type ModelOption } from '$lib/stores/model.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import { updateSession } from '$lib/client/cometmind';
import { matchesShortcut } from '$lib/keyboard-shortcuts';
import type { ChatTurnPayload } from '$lib/actions/start-chat';

export function createChatViewController(deps: {
	getSessionId: () => string;
	getHasVisibleConversation: () => boolean;
	getFirstTurnActive: () => boolean;
	getFirstTurnFlightDone: () => boolean;
	getAwaitingFirstAssistant: () => boolean;
	getForceDocked?: () => boolean;
	enqueue: (payload: ChatTurnPayload | string) => void | Promise<void>;
	cancelTurn: () => void;
}) {
	const canSend = $derived(connectionState.status === 'ready');

	const composerVariant = $derived<'hero' | 'dock'>(
		deps.getForceDocked?.() || shellStore.composerPhase !== 'centered' ? 'dock' : 'hero'
	);

	const heroLayout = $derived(
		!deps.getForceDocked?.() &&
			shellStore.composerPhase === 'centered' &&
			((!deps.getHasVisibleConversation() && !deps.getFirstTurnActive()) ||
				(deps.getFirstTurnActive() && !deps.getFirstTurnFlightDone()))
	);

	function submit(payload: ChatTurnPayload | string) {
		if (!canSend) return;
		void deps.enqueue(payload);
	}

	function stop() {
		deps.cancelTurn();
	}

	const sessionRunActive = $derived(
		chatStore.isStreamingFor(deps.getSessionId()) ||
			(sessionStore.sessions.find((session) => session.id === deps.getSessionId())?.running ??
				false)
	);

	function syncSessionFromStore() {
		const sessionId = deps.getSessionId();
		const session = sessionStore.sessions.find((item) => item.id === sessionId);
		if (!session) return;
		if (sessionStore.current?.id !== sessionId) {
			sessionStore.selectSession(session);
		}
		modelStore.selectFromSession(session);
	}

	function revertModelSelection() {
		const session = sessionStore.sessions.find((item) => item.id === deps.getSessionId());
		if (session) modelStore.selectFromSession(session);
	}

	async function commitModelChange(option: ModelOption) {
		try {
			const updated = await updateSession(deps.getSessionId(), {
				model_id: option.modelId,
				provider_id: option.providerId
			});
			sessionStore.updateSession(updated);
		} catch {
			revertModelSelection();
		}
	}

	async function onModelChange(option: ModelOption) {
		await commitModelChange(option);
	}

	function pollAutonomousTranscript() {
		const id = deps.getSessionId();
		const current = sessionStore.current;
		// User-origin turns already update through SSE/window sync. Only autonomous
		// sessions need polling for transcript writes made by the background worker.
		if (!id || current?.id !== id || current.origin !== 'autonomy') return;
		const interval = window.setInterval(() => {
			if (chatStore.isStreamingFor(id) || chatStore.hasInFlightTurn(id)) return;
			void chatStore.refreshTranscript(id);
		}, 2500);
		return () => window.clearInterval(interval);
	}

	function onStopShortcut(e: KeyboardEvent) {
		if (!matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse)) return;
		if (!sessionRunActive) return;
		const target = e.target;
		if (target instanceof HTMLTextAreaElement || target instanceof HTMLInputElement) {
			if (target.selectionStart !== target.selectionEnd) return;
		}
		e.preventDefault();
		stop();
	}

	return {
		get canSend() {
			return canSend;
		},
		get composerVariant() {
			return composerVariant;
		},
		get heroLayout() {
			return heroLayout;
		},
		get sessionRunActive() {
			return sessionRunActive;
		},
		submit,
		stop,
		syncSessionFromStore,
		onModelChange,
		pollAutonomousTranscript,
		onStopShortcut
	};
}
