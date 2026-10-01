<script lang="ts">
	import { fade } from 'svelte/transition';
	import Composer from '$lib/features/composer/components/Composer.svelte';
	import HeroComposerFrame from '$lib/features/composer/components/HeroComposerFrame.svelte';
	import ChatThread from '$lib/features/chat/components/ChatThread.svelte';
	import FirstTurnFlight from '$lib/features/chat/components/FirstTurnFlight.svelte';
	import UserBubbleFlight from '$lib/features/chat/components/UserBubbleFlight.svelte';
	import {
		createConversationController,
		refreshConversationSession
	} from '$lib/features/chat/conversation-controller';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import { startJobInSession } from '$lib/features/jobs/start-job-in-chat';
	import type { JobResource } from '$lib/client/cometmind';
	import { createChatViewController } from '$lib/features/chat/chat-view-controller.svelte';
	import { createChatViewFlight } from '$lib/features/chat/chat-view-flight.svelte';
	import {
		createChatViewActivation,
		type ChatViewComposerHandle
	} from '$lib/features/chat/chat-view-activation.svelte';
	import { createSessionPhase } from '$lib/features/chat/session-phase.svelte';
	import MiniTitlebar from '$lib/features/chat/components/chat-view/MiniTitlebar.svelte';
	import ChatEmptyRegion from '$lib/features/chat/components/chat-view/ChatEmptyRegion.svelte';

	const THREAD_IN = { duration: 140 };

	let {
		sessionId,
		bootMessage = '',
		compact = false
	}: { sessionId: string; bootMessage?: string; compact?: boolean } = $props();

	const flight = createChatViewFlight({
		getCompact: () => compact,
		getPhase: () => phase,
		getUserBubbleFlight: () => userBubbleFlight,
		getFirstTurnFlight: () => firstTurnFlight
	});

	const conversation = createConversationController({
		getSessionId: () => sessionId,
		send: (sid, payload, opts) => chatStore.send(sid, payload, opts),
		refreshSession: (sid) => refreshConversationSession(sid),
		onQueueChange: syncQueueState,
		onAwaitingFirstAssistantChange: (value) => {
			phase.setAwaitingFirstAssistant(value);
		},
		onTurnRejected: (sid, payload) => activation.restoreRejectedTurn(sid, payload),
		flight: { onUserMessageFlight: flight.onUserMessageFlight }
	});

	let chatHome = $state<HTMLDivElement | null>(null);
	let userBubbleFlight = $state<UserBubbleFlight>();
	let firstTurnFlight = $state<FirstTurnFlight>();
	let queuedCount = $state(0);
	let queuedMessages = $state<QueuedMessage[]>([]);

	// Phase owns snapshot, flight flags, and composer dock. ChatView only
	// calls onSession when the session id changes — before the activation
	// effect can dock on the previous session's visibility.
	const phase = createSessionPhase({
		getSessionId: () => sessionId,
		abortFlights: () => flight.abortFlights(),
		syncQueueState: () => syncQueueState(),
		syncComposerPhase: (opts) => conversation.syncComposerPhase(opts)
	});

	let hasVisibleConversation = $derived(phase.hasVisibleConversation);
	let firstTurnActive = $derived(phase.firstTurnActive);
	let firstTurnFlightDone = $derived(phase.firstTurnFlightDone);
	let firstTurnHandoffPending = $derived(phase.firstTurnHandoffPending);
	let awaitingFirstAssistant = $derived(phase.awaitingFirstAssistant);
	let composerSnap = $derived(chatStore.sessionID === sessionId && chatStore.isLoading);

	const chatView = createChatViewController({
		getSessionId: () => sessionId,
		getHasVisibleConversation: () => phase.hasVisibleConversation,
		getFirstTurnActive: () => phase.firstTurnActive,
		getFirstTurnFlightDone: () => phase.firstTurnFlightDone,
		getAwaitingFirstAssistant: () => phase.awaitingFirstAssistant,
		getForceDocked: () => compact,
		enqueue: (payload) => {
			void conversation.enqueue(payload);
		},
		cancelTurn: () => conversation.cancel()
	});

	let composerVariant = $derived(chatView.composerVariant);
	let heroLayout = $derived(chatView.heroLayout);
	let sessionRunActive = $derived(chatView.sessionRunActive);

	let composerRef = $state<ChatViewComposerHandle | null>(null);

	const activation = createChatViewActivation({
		getSessionId: () => sessionId,
		getComposer: () => composerRef,
		bindSession: () => conversation.bindSession(),
		syncSessionFromStore: () => chatView.syncSessionFromStore(),
		onMount: () => conversation.onMount()
	});

	function syncQueueState() {
		queuedCount = conversation.pendingCount;
		queuedMessages = [...conversation.pendingMessages];
	}

	$effect(() => {
		void sessionStore.sessions;
		chatView.syncSessionFromStore();
	});

	// Soft swaps keep ChatView mounted. One pre-effect calls onSession so stale
	// flight flags clear before composer phase can dock on the previous session.
	$effect.pre(() => {
		phase.onSession(sessionId);
	});

	$effect(() => {
		activation.activate();
	});

	$effect(() => {
		activation.applyComposerFocusRequest();
	});

	function submit(payload: ChatTurnPayload | string) {
		chatView.submit(payload);
	}

	function startJobFromCard(job: JobResource) {
		return startJobInSession(job, sessionId, submit);
	}

	$effect(() => chatView.pollAutonomousTranscript());

	function stop() {
		chatView.stop();
	}

	function removeQueuedMessage(id: string) {
		conversation.removeQueued(id);
	}

	function onWindowFocus() {
		if (!compact) return;
		if (shellStore.focusedPane !== 'chat') return;
		shellStore.requestComposerFocus(sessionId);
	}
</script>

<svelte:window onkeydown={chatView.onStopShortcut} onfocus={onWindowFocus} />

<div
	class="chat-home"
	class:hero-layout={heroLayout}
	class:first-turn-active={firstTurnActive}
	class:compact
	bind:this={chatHome}
>
	{#if compact}
		<MiniTitlebar {sessionId} />
	{/if}

	{#if !compact && !hasVisibleConversation && !firstTurnActive}
		<ChatEmptyRegion {bootMessage} />
	{:else}
		<div
			class="thread-shell"
			class:docked={!heroLayout}
			in:fade={firstTurnActive ? { duration: 0 } : THREAD_IN}
		>
			{#key sessionId}
				<ChatThread
					{sessionId}
					{awaitingFirstAssistant}
					{firstTurnFlightDone}
					{firstTurnHandoffPending}
					onNotifyAgent={submit}
					onStartJob={startJobFromCard}
				/>
			{/key}
		</div>
	{/if}

	<UserBubbleFlight
		bind:this={userBubbleFlight}
		root={chatHome}
		stageUser={(text, images) => chatStore.stageUserForSession(sessionId, text, images)}
		revealStagedUser={() => chatStore.revealStagedUserForSession(sessionId)}
	/>

	<FirstTurnFlight
		bind:this={firstTurnFlight}
		root={chatHome}
		{userBubbleFlight}
		stageUser={(text, images) => chatStore.stageUserForSession(sessionId, text, images)}
		revealStagedUser={() => chatStore.revealStagedUserForSession(sessionId)}
		onActiveChange={(active) => phase.setFirstTurnActive(active)}
		onPrepareFlight={() => {
			shellStore.dockComposer();
		}}
		onFlightDoneChange={(done) => {
			phase.setFirstTurnFlightDone(done);
			phase.setFirstTurnHandoffPending(!done);
		}}
	/>

	<div
		class="composer-wrapper"
		class:centered={!compact && shellStore.composerPhase === 'centered'}
		class:snap={composerSnap}
	>
		<HeroComposerFrame active={composerVariant === 'hero'}>
			<Composer
				bind:this={composerRef}
				onSend={submit}
				onStop={stop}
				onRemoveQueued={removeQueuedMessage}
				onModelChange={chatView.onModelChange}
				onWorkspaceChanged={() => chatStore.loadTranscript(sessionId)}
				onTranscriptCleared={() => {
					conversation.clearQueue();
					syncQueueState();
				}}
				{sessionId}
				disabled={!chatView.canSend}
				streaming={sessionRunActive}
				{queuedCount}
				{queuedMessages}
				variant={composerVariant}
			/>
		</HeroComposerFrame>
	</div>
</div>

<style>
	.chat-home {
		position: relative;
		flex: 1;
		min-height: 0;
		width: 100%;
		overflow: hidden;
	}

	.chat-home.compact {
		flex: none;
		height: 100vh;
		min-height: 100vh;
		--mini-titlebar-height: 46px;
		--user-message-collapsed-height: min(15rem, 36dvh);
		background:
			radial-gradient(
				circle at top,
				color-mix(in srgb, var(--hero-composer-glow-color) 16%, transparent),
				transparent 42%
			),
			var(--app-bg);
	}

	.chat-home.compact .thread-shell,
	.chat-home.compact .composer-wrapper,
	.chat-home.compact :global(button),
	.chat-home.compact :global(input),
	.chat-home.compact :global(textarea),
	.chat-home.compact :global(select),
	.chat-home.compact :global(a),
	.chat-home.compact :global([role='button']) {
		-webkit-app-region: no-drag;
	}

	.chat-home.hero-layout {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		place-items: center;
		align-content: center;
		gap: clamp(1.5rem, 6cqi, 52px);
		padding: clamp(1rem, 5cqi, 48px);
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
	}

	.chat-home.hero-layout .composer-wrapper {
		position: relative;
		bottom: auto;
		left: auto;
		transform: none;
		width: 100%;
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
		padding: 0 var(--chat-gutter);
		display: flex;
		justify-content: center;
	}

	.thread-shell {
		position: absolute;
		inset: 0;
		transition: bottom var(--duration-flight) var(--ease-smooth);
	}

	.thread-shell.docked {
		bottom: var(--thread-dock-inset);
	}

	.chat-home.compact .thread-shell.docked {
		top: var(--mini-titlebar-height);
		bottom: calc(var(--thread-dock-inset) - 18px);
	}

	.composer-wrapper {
		position: absolute;
		left: 0;
		width: 100%;
		z-index: 10;
		padding: 0 var(--chat-gutter);
		display: flex;
		justify-content: center;
		overflow: visible;
		transition:
			bottom var(--duration-flight) var(--ease-smooth),
			transform var(--duration-flight) var(--ease-smooth);
	}

	.composer-wrapper.snap {
		transition: none;
	}

	.composer-wrapper.centered {
		bottom: var(--composer-hero-bottom);
		transform: translateY(50%);
	}

	.composer-wrapper:not(.centered) {
		bottom: var(--composer-dock-bottom);
		transform: none;
	}

	.chat-home.compact .composer-wrapper {
		padding-inline: 14px;
	}

	.composer-wrapper :global(.hero-composer-frame) {
		width: min(var(--chat-composer-width), 100%);
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
	}

	@media (max-width: 900px) {
		.chat-home.hero-layout {
			gap: 40px;
			padding: 32px 28px;
		}
	}
</style>
