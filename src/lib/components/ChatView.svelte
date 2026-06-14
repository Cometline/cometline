<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import ChatThread from '$lib/components/ChatThread.svelte';
	import FirstTurnFlight from '$lib/components/FirstTurnFlight.svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { getSession } from '$lib/client/cometmind';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { startChat } from '$lib/actions/start-chat';
	import { createChatTurnQueue, type QueuedMessage } from '$lib/actions/chat-turn-queue';

	const THREAD_IN = { duration: 180 };

	// This component is keyed on sessionId by the route, so it remounts per
	// session and sessionId is constant for the instance's lifetime. That lets
	// per-session work live in onMount instead of sessionId-watching effects.
	let { sessionId, bootMessage = '' }: { sessionId: string; bootMessage?: string } = $props();

	let chatHome = $state<HTMLDivElement | null>(null);
	let userBubbleFlight = $state<UserBubbleFlight>();
	let firstTurnFlight = $state<FirstTurnFlight>();
	let awaitingFirstAssistant = $state(false);
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);
	let queuedCount = $state(0);
	let queuedMessages = $state<QueuedMessage[]>([]);

	let hasVisibleConversation = $derived(chatStore.items.length > 0 || chatStore.isLoading);
	let composerVariant = $derived<'hero' | 'dock'>(
		shellStore.composerPhase === 'centered' ? 'hero' : 'dock'
	);
	let heroLayout = $derived(
		shellStore.composerPhase === 'centered' &&
			((!hasVisibleConversation && !firstTurnActive) ||
				(firstTurnActive && !firstTurnFlightDone))
	);

	let turnProcessing = $state(false);

	function syncQueueState() {
		queuedCount = turnQueue?.pendingCount ?? 0;
		queuedMessages = turnQueue ? [...turnQueue.pendingMessages] : [];
		turnProcessing = turnQueue?.processing ?? false;
	}

	const turnQueue = createChatTurnQueue(async (text) => {
		await startChat(
			{
				get sessionId() {
					return sessionId;
				},
				get hasVisibleConversation() {
					return hasVisibleConversation;
				},
				send: (t, opts) => chatStore.send(sessionId, t, opts),
				onUserMessageFlight: (text, { firstTurn }) => {
					if (firstTurn) {
						awaitingFirstAssistant = true;
						firstTurnFlight?.run(text);
						return;
					}
					userBubbleFlight?.run(text);
				},
				onFirstTurnComplete: () => {
					awaitingFirstAssistant = false;
				},
				refreshSession
			},
			text
		);
	}, syncQueueState);

	onMount(() => {
		// Select this session and sync the model picker to it.
		const session = sessionStore.sessions.find((item) => item.id === sessionId);
		if (session) {
			sessionStore.selectSession(session);
			modelStore.selectFromSession(session);
		}

		// A pending first message (queued by the composer before navigation) takes
		// priority and is submitted as the first turn; otherwise load the
		// transcript for an existing session.
		const pending = sessionStore.takePendingMessage(sessionId);
		if (pending) {
			submit(pending);
		} else if (!(chatStore.isStreaming && chatStore.sessionID === sessionId)) {
			void chatStore.loadTranscript(sessionId).then(() => {
				if (chatStore.items.length > 0) shellStore.dockComposer();
				else shellStore.centerComposer();
			});
		}
	});

	$effect(() => {
		if (!hasVisibleConversation && !firstTurnActive) {
			awaitingFirstAssistant = false;
			firstTurnFlightDone = false;
		}
	});

	function submit(text: string) {
		if (connectionState.status !== 'ready') return;
		void turnQueue.enqueue(text);
	}

	function stop() {
		void chatStore.cancel(sessionId);
	}

	function removeQueuedMessage(id: string) {
		turnQueue.remove(id);
	}

	function onWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'c' || !(e.ctrlKey || e.metaKey)) return;
		if (!chatStore.isStreaming) return;
		const target = e.target;
		if (target instanceof HTMLTextAreaElement || target instanceof HTMLInputElement) {
			if (target.selectionStart !== target.selectionEnd) return;
		}
		e.preventDefault();
		stop();
	}

	async function refreshSession() {
		try {
			sessionStore.updateSession(await getSession(sessionId));
		} catch {
			// The transcript is the source of truth; title refresh is best effort.
		}
	}
</script>

<svelte:window onkeydown={onWindowKeydown} />

<div
	class="chat-home"
	class:hero-layout={heroLayout}
	class:first-turn-active={firstTurnActive}
	bind:this={chatHome}
>
	{#if !hasVisibleConversation && !firstTurnActive}
		<div class="empty-region">
			<EmptyChatState />
			{#if bootMessage}
				<p class="boot-error">{bootMessage}</p>
			{/if}
		</div>
	{:else}
		<div
			class="thread-shell"
			class:docked={!heroLayout}
			in:fade={firstTurnActive ? { duration: 0 } : THREAD_IN}
		>
			<ChatThread {awaitingFirstAssistant} {firstTurnFlightDone} />
		</div>
	{/if}

	<UserBubbleFlight
		bind:this={userBubbleFlight}
		root={chatHome}
		stageUser={(text) => chatStore.stageUser(text)}
		revealStagedUser={() => chatStore.revealStagedUser()}
	/>

	<FirstTurnFlight
		bind:this={firstTurnFlight}
		root={chatHome}
		{userBubbleFlight}
		stageUser={(text) => chatStore.stageUser(text)}
		revealStagedUser={() => chatStore.revealStagedUser()}
		onActiveChange={(active) => (firstTurnActive = active)}
		onPrepareFlight={() => shellStore.dockComposer()}
		onFlightDoneChange={(done) => {
			firstTurnFlightDone = done;
		}}
	/>

	<div class="composer-wrapper" class:centered={shellStore.composerPhase === 'centered'}>
		<Composer
			onSend={submit}
			onStop={stop}
			onRemoveQueued={removeQueuedMessage}
			disabled={connectionState.status !== 'ready'}
			streaming={chatStore.isStreaming}
			{queuedCount}
			{queuedMessages}
			waitingForReply={chatStore.isStreaming || firstTurnActive}
			turnProcessing={turnProcessing}
			variant={composerVariant}
		/>
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

	.chat-home.hero-layout {
		display: grid;
		place-items: center;
		align-content: center;
		gap: 52px;
		padding: 48px;
	}

	.chat-home.hero-layout .empty-region {
		position: static;
		inset: unset;
		padding: 0;
	}

	.empty-region {
		position: absolute;
		inset: 0 0 180px;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 48px 48px 0;
		flex-direction: column;
	}

	.thread-shell {
		position: absolute;
		inset: 0;
		transition: bottom var(--duration-flight) var(--ease-smooth);
	}

	.thread-shell.docked {
		bottom: var(--thread-dock-inset);
	}

	.boot-error {
		margin: 18px 0 0;
		max-width: 520px;
		font-size: 12px;
		line-height: 1.5;
		color: #b42318;
		text-align: center;
	}

	.composer-wrapper {
		position: absolute;
		left: 0;
		width: 100%;
		z-index: 10;
		padding: 0 var(--chat-gutter);
		display: flex;
		justify-content: center;
		transition:
			bottom var(--duration-flight) var(--ease-smooth),
			transform var(--duration-flight) var(--ease-smooth);
	}

	.composer-wrapper.centered {
		bottom: var(--composer-hero-bottom);
		transform: translateY(50%);
	}

	.composer-wrapper:not(.centered) {
		bottom: var(--composer-dock-bottom);
		transform: none;
	}

	.composer-wrapper :global(.composer) {
		width: min(var(--chat-composer-width), 100%);
		max-width: 100%;
	}

	@media (max-width: 900px) {
		.chat-home.hero-layout {
			gap: 40px;
			padding: 32px 28px;
		}

		.empty-region {
			inset: 0 0 160px;
			padding-inline: 28px;
		}
	}
</style>
