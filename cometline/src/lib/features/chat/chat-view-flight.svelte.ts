import type FirstTurnFlight from '$lib/features/chat/components/FirstTurnFlight.svelte';
import type UserBubbleFlight from '$lib/features/chat/components/UserBubbleFlight.svelte';
import type { ConversationFlightAdapter } from '$lib/features/chat/conversation-controller';
import type { createSessionPhase } from '$lib/features/chat/session-phase.svelte';

type SessionPhase = ReturnType<typeof createSessionPhase>;

export function createChatViewFlight(deps: {
	getCompact: () => boolean;
	getPhase: () => SessionPhase;
	getUserBubbleFlight: () => UserBubbleFlight | undefined;
	getFirstTurnFlight: () => FirstTurnFlight | undefined;
}) {
	let flightAbortController = $state<AbortController | null>(null);

	function startFlight() {
		flightAbortController?.abort();
		flightAbortController = new AbortController();
		return flightAbortController;
	}

	function finishFlight(controller: AbortController) {
		if (flightAbortController === controller) flightAbortController = null;
	}

	function abortFlights() {
		flightAbortController?.abort();
		flightAbortController = null;
		deps.getFirstTurnFlight()?.cancel();
		deps.getUserBubbleFlight()?.dismissParticle();
	}

	const onUserMessageFlight: ConversationFlightAdapter['onUserMessageFlight'] = (
		payloadOrText,
		{ firstTurn, stageUser, revealStagedUser }
	) => {
		const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
		const phase = deps.getPhase();
		const userBubbleFlight = deps.getUserBubbleFlight();
		const firstTurnFlight = deps.getFirstTurnFlight();
		// Follow-ups are handled by the conversation controller (short fade + pin).
		// This adapter only owns first-turn choreography (desktop + mini).
		if (!firstTurn) return;
		if (deps.getCompact()) {
			phase.setAwaitingFirstAssistant(true);
			// Mini uses the regular user-bubble flight rather than the desktop
			// first-turn choreography. Keep its destination independent of the
			// first assistant's lifecycle so it is revealed after the flight.
			phase.setFirstTurnFlightDone(true);
			phase.setFirstTurnHandoffPending(false);
			if (!userBubbleFlight) {
				stageUser(payload.text, payload.images);
				revealStagedUser();
				return;
			}
			const compactUserItemId = stageUser(payload.text, payload.images);
			const flight = startFlight();
			return userBubbleFlight
				.runAsync(payload.text, payload.images, {
					origin: 'above-composer',
					skipStage: true,
					targetUserId: compactUserItemId,
					signal: flight.signal
				})
				.then(() => undefined)
				.finally(() => finishFlight(flight));
		}
		phase.setAwaitingFirstAssistant(true);
		phase.setFirstTurnFlightDone(false);
		phase.setFirstTurnHandoffPending(true);
		if (!firstTurnFlight) {
			phase.setFirstTurnFlightDone(true);
			phase.setFirstTurnHandoffPending(false);
			stageUser(payload.text, payload.images);
			revealStagedUser();
			return;
		}
		const flight = startFlight();
		return firstTurnFlight
			?.runAsync(payload.text, payload.images, {
				stageUser,
				revealStagedUser,
				signal: flight.signal
			})
			.catch((error) => {
				phase.setFirstTurnFlightDone(true);
				phase.setFirstTurnHandoffPending(false);
				throw error;
			})
			.finally(() => finishFlight(flight));
	};

	return { onUserMessageFlight, abortFlights };
}
