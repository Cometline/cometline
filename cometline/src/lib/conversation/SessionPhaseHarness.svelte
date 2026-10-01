<script lang="ts">
	import { createConversationController } from './conversation-controller';
	import { createSessionPhase } from './session-phase.svelte';

	let { sessionId }: { sessionId: string } = $props();

	const conversation = createConversationController({
		getSessionId: () => sessionId,
		send: async () => {},
		refreshSession: async () => {}
	});

	const phase = createSessionPhase({
		getSessionId: () => sessionId,
		abortFlights: () => {},
		syncQueueState: () => {},
		syncComposerPhase: (opts) => conversation.syncComposerPhase(opts)
	});

	// Same seam as ChatView: session id is the only dependency.
	$effect.pre(() => {
		phase.onSession(sessionId);
	});
</script>

<div
	data-testid="session-phase"
	data-visible={phase.hasVisibleConversation}
	data-flight-done={phase.firstTurnFlightDone}
	data-handoff={phase.firstTurnHandoffPending}
	data-synced={phase.snapshotSynced}
	data-awaiting={phase.awaitingFirstAssistant}
></div>
