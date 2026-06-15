<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import HeroComposerFrame from '$lib/components/HeroComposerFrame.svelte';
	import ChatThread from '$lib/components/ChatThread.svelte';
	import FirstTurnFlight from '$lib/components/FirstTurnFlight.svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { getSession, updateSession } from '$lib/client/cometmind';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { startChat } from '$lib/actions/start-chat';
	import { createChatTurnQueue, type QueuedMessage } from '$lib/actions/chat-turn-queue';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import type { ImageAttachment } from '$lib/types';
	import type { ModelOption } from '$lib/stores/model.svelte';

	const THREAD_IN = { duration: 180 };

	// This component is keyed on sessionId by the route, so it remounts per
	// session and sessionId is constant for the instance's lifetime. That lets
	// per-session work live in onMount instead of sessionId-watching effects.
	let { sessionId, bootMessage = '' }: { sessionId: string; bootMessage?: string } = $props();

	$effect.pre(() => {
		chatStore.bindSession(sessionId);
		if (sessionStore.hasPendingMessage(sessionId)) return;
		if (chatStore.isStreaming && chatStore.sessionID === sessionId) return;
		void chatStore.loadTranscript(sessionId);
	});

	let chatHome = $state<HTMLDivElement | null>(null);
	let userBubbleFlight = $state<UserBubbleFlight>();
	let firstTurnFlight = $state<FirstTurnFlight>();
	let awaitingFirstAssistant = $state(false);
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);
	let queuedCount = $state(0);
	let queuedMessages = $state<QueuedMessage[]>([]);

	let hasVisibleConversation = $derived(
		chatStore.sessionID === sessionId && (chatStore.items.length > 0 || chatStore.isLoading)
	);
	let composerVariant = $derived<'hero' | 'dock'>(
		shellStore.composerPhase === 'centered' ? 'hero' : 'dock'
	);
	let heroLayout = $derived(
		shellStore.composerPhase === 'centered' &&
			((!hasVisibleConversation && !firstTurnActive) ||
				(firstTurnActive && !firstTurnFlightDone))
	);

	let heroFrameExiting = $state(false);
	let turnProcessing = $state(false);

	function syncQueueState() {
		queuedCount = turnQueue?.pendingCount ?? 0;
		queuedMessages = turnQueue ? [...turnQueue.pendingMessages] : [];
		turnProcessing = turnQueue?.processing ?? false;
	}

	const turnQueue = createChatTurnQueue(async (text, images) => {
		await startChat(
			{
				get sessionId() {
					return sessionId;
				},
				get hasVisibleConversation() {
					return hasVisibleConversation;
				},
				send: (payload, opts) => chatStore.send(sessionId, payload, opts),
				onUserMessageFlight: (payloadOrText, { firstTurn }) => {
					const payload =
						typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
					if (firstTurn) {
						awaitingFirstAssistant = true;
						firstTurnFlight?.run(payload.text, payload.images);
						return;
					}
					userBubbleFlight?.run(payload.text, payload.images);
				},
				onFirstTurnComplete: () => {
					awaitingFirstAssistant = false;
				},
				refreshSession
			},
			{ text, images }
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
			submit(pending.text, pending.images);
		} else {
			void chatStore.loadTranscript(sessionId);
		}
	});

	$effect(() => {
		if (chatStore.sessionID !== sessionId) return;
		if (firstTurnActive) return;

		if (hasVisibleConversation) {
			shellStore.dockComposer();
		} else if (!chatStore.isLoading) {
			shellStore.centerComposer();
		}
	});

	$effect(() => {
		if (!hasVisibleConversation && !firstTurnActive) {
			awaitingFirstAssistant = false;
			firstTurnFlightDone = false;
			heroFrameExiting = false;
		}
	});

	function submit(text: string, images?: ImageAttachment[]) {
		if (connectionState.status !== 'ready') return;
		void turnQueue.enqueue(text, images);
	}

	function stop() {
		void chatStore.cancel(sessionId);
	}

	function removeQueuedMessage(id: string) {
		turnQueue.remove(id);
	}

	function onWindowKeydown(e: KeyboardEvent) {
		if (!matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse)) return;
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

	async function onModelChange(option: ModelOption) {
		try {
			const updated = await updateSession(sessionId, {
				model_id: option.modelId,
				provider_id: option.providerId
			});
			sessionStore.updateSession(updated);
		} catch {
			const session = sessionStore.sessions.find((item) => item.id === sessionId);
			if (session) modelStore.selectFromSession(session);
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
			<ChatThread {sessionId} {awaitingFirstAssistant} {firstTurnFlightDone} />
		</div>
	{/if}

	<UserBubbleFlight
		bind:this={userBubbleFlight}
		root={chatHome}
		stageUser={(text, images) => chatStore.stageUser(text, images)}
		revealStagedUser={() => chatStore.revealStagedUser()}
	/>

	<FirstTurnFlight
		bind:this={firstTurnFlight}
		root={chatHome}
		{userBubbleFlight}
		stageUser={(text, images) => chatStore.stageUser(text, images)}
		revealStagedUser={() => chatStore.revealStagedUser()}
		onActiveChange={(active) => (firstTurnActive = active)}
		onPrepareFlight={() => {
			if (composerVariant === 'hero') heroFrameExiting = true;
			shellStore.dockComposer();
		}}
		onFlightDoneChange={(done) => {
			firstTurnFlightDone = done;
		}}
	/>

	<div class="composer-wrapper" class:centered={shellStore.composerPhase === 'centered'}>
		<HeroComposerFrame
			active={composerVariant === 'hero' && !heroFrameExiting}
			exiting={heroFrameExiting}
			onExitComplete={() => {
				heroFrameExiting = false;
			}}
		>
			<Composer
				onSend={submit}
				onStop={stop}
				onRemoveQueued={removeQueuedMessage}
				onModelChange={onModelChange}
				disabled={connectionState.status !== 'ready'}
				streaming={chatStore.isStreaming}
				{queuedCount}
				{queuedMessages}
				waitingForReply={chatStore.isStreaming || firstTurnActive}
				{turnProcessing}
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

	.chat-home.hero-layout .composer-wrapper {
		position: relative;
		bottom: auto;
		left: auto;
		transform: none;
		width: 100%;
		padding: 0 var(--chat-gutter);
		display: flex;
		justify-content: center;
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
		overflow: visible;
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

	.composer-wrapper :global(.hero-composer-frame) {
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
