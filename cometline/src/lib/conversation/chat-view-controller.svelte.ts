import { connectionState } from '$lib/stores/runtime.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import type { ChatTurnPayload } from '$lib/actions/start-chat';

export function createChatViewController(deps: {
	getSessionId: () => string;
	getHasVisibleConversation: () => boolean;
	getFirstTurnActive: () => boolean;
	getFirstTurnFlightDone: () => boolean;
	getAwaitingFirstAssistant: () => boolean;
	enqueue: (payload: ChatTurnPayload | string) => void | Promise<void>;
	cancelTurn: () => void;
}) {
	const canSend = $derived(connectionState.status === 'ready');

	const composerVariant = $derived<'hero' | 'dock'>(
		shellStore.composerPhase === 'centered' ? 'hero' : 'dock'
	);

	const heroLayout = $derived(
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
		submit,
		stop
	};
}
