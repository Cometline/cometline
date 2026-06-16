<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import HeroComposerFrame from '$lib/components/HeroComposerFrame.svelte';
	import ChatThread from '$lib/components/ChatThread.svelte';
	import FirstTurnFlight from '$lib/components/FirstTurnFlight.svelte';
	import UserBubbleFlight from '$lib/components/UserBubbleFlight.svelte';
	import {
		createConversationController,
		refreshConversationSession
	} from '$lib/conversation/conversation-controller';
	import type { QueuedMessage } from '$lib/actions/chat-turn-queue';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { updateSession } from '$lib/client/cometmind';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import type { ImageAttachment, ChatItem } from '$lib/types';
	import type { ModelOption } from '$lib/stores/model.svelte';

	const THREAD_IN = { duration: 140 };

	let { sessionId, bootMessage = '' }: { sessionId: string; bootMessage?: string } = $props();

	const conversation = createConversationController({
		getSessionId: () => sessionId,
		getHasVisibleConversation: () => hasVisibleConversation,
		send: (payload, opts) => chatStore.send(sessionId, payload, opts),
		refreshSession: () => refreshConversationSession(sessionId),
		onQueueChange: syncQueueState,
		onAwaitingFirstAssistantChange: (value) => {
			awaitingFirstAssistant = value;
		},
		flight: {
			onUserMessageFlight: (payloadOrText, { firstTurn }) => {
				const payload =
					typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
				if (firstTurn) {
					awaitingFirstAssistant = true;
					firstTurnFlight?.run(payload.text, payload.images);
					return;
				}
				userBubbleFlight?.run(payload.text, payload.images, {
					origin: 'above-composer'
				});
			}
		}
	});

	$effect.pre(() => {
		conversation.bindSession();
		if (!conversation.shouldSkipTranscriptLoad()) {
			void chatStore.loadTranscript(sessionId);
		}
	});

	let chatHome = $state<HTMLDivElement | null>(null);
	let userBubbleFlight = $state<UserBubbleFlight>();
	let firstTurnFlight = $state<FirstTurnFlight>();
	let awaitingFirstAssistant = $state(false);
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);
	let queuedCount = $state(0);
	let queuedMessages = $state<QueuedMessage[]>([]);

	let snapshotItems = $state.raw<ChatItem[]>([]);
	let snapshotLoading = $state(false);

	$effect(() => {
		if (chatStore.sessionID !== sessionId) return;
		snapshotItems = chatStore.items;
		snapshotLoading = chatStore.isLoading;
	});

	let hasVisibleConversation = $derived.by(() => {
		if (firstTurnActive || awaitingFirstAssistant) return true;
		if (chatStore.sessionID === sessionId) {
			return chatStore.items.length > 0 || chatStore.isLoading;
		}
		return snapshotItems.length > 0 || snapshotLoading;
	});
	let composerSnap = $derived(
		chatStore.sessionID === sessionId && chatStore.isLoading
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

	function syncQueueState() {
		queuedCount = conversation.pendingCount;
		queuedMessages = [...conversation.pendingMessages];
	}

	function syncSessionFromStore() {
		const session = sessionStore.sessions.find((item) => item.id === sessionId);
		if (!session) return;
		if (sessionStore.current?.id !== sessionId) {
			sessionStore.selectSession(session);
		}
		modelStore.selectFromSession(session);
	}

	$effect(() => {
		sessionStore.sessions;
		syncSessionFromStore();
	});

	onMount(() => {
		syncSessionFromStore();
		conversation.onMount();
	});

	$effect(() => {
		conversation.syncComposerPhase({
			hasVisibleConversation,
			firstTurnActive,
			awaitingFirstAssistant
		});
	});

	$effect(() => {
		if (!hasVisibleConversation && !firstTurnActive && !awaitingFirstAssistant) {
			firstTurnFlightDone = false;
			heroFrameExiting = false;
		}
	});

	function submit(text: string, images?: ImageAttachment[], filePaths?: string[]) {
		if (connectionState.status !== 'ready') return;
		void conversation.enqueue(text, images, filePaths);
	}

	function stop() {
		conversation.cancel();
	}

	function removeQueuedMessage(id: string) {
		conversation.removeQueued(id);
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

	<div
		class="composer-wrapper"
		class:centered={shellStore.composerPhase === 'centered'}
		class:snap={composerSnap}
	>
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
				onWorkspaceChanged={() => chatStore.loadTranscript(sessionId)}
				{sessionId}
				disabled={connectionState.status !== 'ready'}
				streaming={chatStore.isStreaming}
				{queuedCount}
				{queuedMessages}
				waitingForReply={chatStore.isStreaming || firstTurnActive}
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
