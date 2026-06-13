<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import ChatThread from '$lib/components/ChatThread.svelte';
	import FirstTurnFlight from '$lib/components/FirstTurnFlight.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { getSession } from '$lib/client/cometmind';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { startChat } from '$lib/actions/start-chat';

	const THREAD_IN = { duration: 180 };

	// This component is keyed on sessionId by the route, so it remounts per
	// session and sessionId is constant for the instance's lifetime. That lets
	// per-session work live in onMount instead of sessionId-watching effects.
	let { sessionId, bootMessage = '' }: { sessionId: string; bootMessage?: string } = $props();

	let chatHome = $state<HTMLDivElement | null>(null);
	let firstTurnFlight: FirstTurnFlight;
	let awaitingFirstAssistant = $state(false);
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);

	let hasVisibleConversation = $derived(chatStore.items.length > 0 || chatStore.isLoading);
	let composerVariant = $derived<'hero' | 'dock'>(shellStore.composerPhase === 'centered' ? 'hero' : 'dock');
	let heroLayout = $derived(
		!hasVisibleConversation && shellStore.composerPhase === 'centered' && !firstTurnActive
	);

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
			void submit(pending);
		} else if (!(chatStore.isStreaming && chatStore.sessionID === sessionId)) {
			void chatStore.loadTranscript(sessionId);
		}
	});

	$effect(() => {
		if (!hasVisibleConversation && !firstTurnActive) {
			awaitingFirstAssistant = false;
			firstTurnFlightDone = false;
		}
	});

	$effect(() => {
		if (hasVisibleConversation && !firstTurnActive) {
			shellStore.dockComposer();
		}
	});

	async function submit(text: string) {
		await startChat(
			{
				get sessionId() {
					return sessionId;
				},
				get hasVisibleConversation() {
					return hasVisibleConversation;
				},
				send: (t, opts) => chatStore.send(sessionId, t, opts),
				onFirstTurnStart: async (text) => {
					awaitingFirstAssistant = true;
					shellStore.dockComposer();
					firstTurnFlight.run(text);
				},
				onFirstTurnComplete: () => {
					awaitingFirstAssistant = false;
				},
				refreshSession
			},
			text
		);
	}

	async function refreshSession() {
		try {
			sessionStore.updateSession(await getSession(sessionId));
		} catch {
			// The transcript is the source of truth; title refresh is best effort.
		}
	}
</script>

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
		<div class="thread-shell" transition:fade={THREAD_IN}>
			<ChatThread {awaitingFirstAssistant} {firstTurnFlightDone} />
		</div>
	{/if}

	<FirstTurnFlight
		bind:this={firstTurnFlight}
		root={chatHome}
		stageUser={(text) => chatStore.stageUser(text)}
		revealStagedUser={() => chatStore.revealStagedUser()}
		onActiveChange={(active) => (firstTurnActive = active)}
		onFlightDoneChange={(done) => (firstTurnFlightDone = done)}
	/>

	<div class="composer-wrapper" class:centered={shellStore.composerPhase === 'centered'}>
		<Composer
			onSend={submit}
			disabled={chatStore.isStreaming || connectionState.status !== 'ready'}
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
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 36px;
		padding: 48px;
	}

	.chat-home.hero-layout .empty-region {
		position: static;
		inset: unset;
		flex: 0 0 auto;
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
		left: 50%;
		transform: translateX(-50%);
		width: min(var(--composer-width), calc(100% - 64px));
		z-index: 10;
		bottom: 40px;
		transition:
			bottom var(--duration-flight) var(--ease-smooth),
			transform var(--duration-flight) var(--ease-smooth),
			width var(--duration-flight) var(--ease-smooth);
	}

	.composer-wrapper.centered {
		bottom: 50%;
		transform: translateX(-50%) translateY(168px);
		width: min(700px, calc(100% - 64px));
	}

	@media (max-width: 900px) {
		.composer-wrapper {
			bottom: 24px;
			width: calc(100% - 40px);
		}

		.chat-home.hero-layout {
			gap: 28px;
			padding: 32px 28px;
		}

		.empty-region {
			inset: 0 0 160px;
			padding-inline: 28px;
		}
	}
</style>
