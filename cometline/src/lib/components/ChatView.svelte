<script lang="ts">
	import { fade } from 'svelte/transition';
	import { tick, untrack } from 'svelte';
	import EmptyChatState from '$lib/components/EmptyChatState.svelte';
	import Composer from '$lib/components/composer/Composer.svelte';
	import HeroComposerFrame from '$lib/components/HeroComposerFrame.svelte';
	import ChatThread from '$lib/components/chat/ChatThread.svelte';
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
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import type { ChatItem } from '$lib/types';
	import type { ModelOption } from '$lib/stores/model.svelte';
	import ProviderSwitchDialog from '$lib/components/ProviderSwitchDialog.svelte';
	import { analyzeProviderSwitch, type ProviderSwitchWarning } from '$lib/provider-switch';

	const THREAD_IN = { duration: 140 };

	let { sessionId, bootMessage = '' }: { sessionId: string; bootMessage?: string } = $props();

	const conversation = createConversationController({
		getSessionId: () => sessionId,
		getHasVisibleConversation: () => hasVisibleConversation,
		send: (sid, payload, opts) => chatStore.send(sid, payload, opts),
		refreshSession: (sid) => refreshConversationSession(sid),
		onQueueChange: syncQueueState,
		onAwaitingFirstAssistantChange: (value) => {
			awaitingFirstAssistant = value;
		},
		flight: {
			onUserMessageFlight: (payloadOrText, { firstTurn, stageUser, revealStagedUser }) => {
				const payload =
					typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
				if (firstTurn) {
					awaitingFirstAssistant = true;
					firstTurnFlightDone = false;
					firstTurnHandoffPending = true;
					if (!firstTurnFlight) {
						firstTurnFlightDone = true;
						firstTurnHandoffPending = false;
						return;
					}
					return firstTurnFlight?.runAsync(payload.text, payload.images, {
						stageUser,
						revealStagedUser
					}).catch((error) => {
						firstTurnFlightDone = true;
						firstTurnHandoffPending = false;
						throw error;
					});
				}
				return userBubbleFlight
					?.runAsync(payload.text, payload.images, {
						origin: 'above-composer',
						skipStage: true,
						skipReveal: true
					})
					.then(() => undefined);
			}
		}
	});

	let chatHome = $state<HTMLDivElement | null>(null);
	let userBubbleFlight = $state<UserBubbleFlight>();
	let firstTurnFlight = $state<FirstTurnFlight>();
	let awaitingFirstAssistant = $state(false);
	let firstTurnActive = $state(false);
	let firstTurnFlightDone = $state(false);
	let firstTurnHandoffPending = $state(false);
	let queuedCount = $state(0);
	let queuedMessages = $state<QueuedMessage[]>([]);
	let turnBusy = $state(false);
	let pendingSwitch = $state<{ option: ModelOption; warnings: ProviderSwitchWarning[] } | null>(
		null
	);

	let snapshotItems = $state.raw<ChatItem[]>([]);
	// Default to "loading" until the store binds this session so a freshly
	// mounted ChatView (e.g. switching sessions while a previous transcript is
	// still in flight) shows the loading state instead of flashing the empty
	// state and then getting stuck without messages.
	let snapshotLoading = $state(true);
	let snapshotSynced = $state(false);

	$effect(() => {
		if (chatStore.sessionID !== sessionId) return;
		snapshotItems = chatStore.items;
		snapshotLoading = chatStore.isLoading;
		snapshotSynced = true;
	});

	let hasVisibleConversation = $derived.by(() => {
		if (firstTurnActive || awaitingFirstAssistant) return true;
		if (chatStore.sessionID === sessionId) {
			return chatStore.items.length > 0 || chatStore.isLoading;
		}
		// Store is still bound to a previous session (mid-switch). Before our
		// first sync, assume loading so we don't flash the empty state.
		if (!snapshotSynced) return true;
		return snapshotItems.length > 0 || snapshotLoading;
	});
	let composerSnap = $derived(chatStore.sessionID === sessionId && chatStore.isLoading);
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
		turnBusy = conversation.processing;
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
		void sessionStore.sessions;
		syncSessionFromStore();
	});

	// Reset per-session view state ONLY when the active session changes. The chat
	// store reads below must be untracked: otherwise staging the user message and
	// adding the pending assistant row during a first-turn flight re-runs this
	// effect, which would reset firstTurnHandoffPending mid-flight and let the
	// destination avatar/thinking indicator appear before the overlay arrives.
	$effect(() => {
		void sessionId;
		untrack(() => {
			firstTurnActive = false;
			firstTurnHandoffPending = false;
			heroFrameExiting = false;
			snapshotSynced = false;
			snapshotLoading = true;
			awaitingFirstAssistant = chatStore.isAwaitingFirstAssistant(sessionId);
			// Returning to a session with cached transcript should show avatars
			// immediately; only a live empty-state first-turn flight hides them.
			firstTurnFlightDone =
				chatStore.getCachedItemCount(sessionId) > 0 || !awaitingFirstAssistant;
			syncQueueState();
		});
	});

	let activatedSessionId = $state<string | null>(null);
	let activationRun = 0;

	let composerRef = $state<{ focus: () => void } | null>(null);

	async function activateSession(id: string, run: number) {
		await tick();
		if (activationRun !== run || sessionId !== id) return;
		conversation.onMount();
		setTimeout(() => composerRef?.focus(), 0);
	}

	$effect(() => {
		if (!sessionId) return;
		if (activatedSessionId === sessionId) return;
		activatedSessionId = sessionId;
		const run = ++activationRun;
		conversation.bindSession();
		syncSessionFromStore();
		void activateSession(sessionId, run);
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

	function submit(payload: ChatTurnPayload | string) {
		if (connectionState.status !== 'ready') return;
		void conversation.enqueue(payload);
	}

	function showLocalUserMessage(text: string) {
		chatStore.appendLocalUserMessage(sessionId, text);
	}

	function stop() {
		conversation.cancel();
	}

	function removeQueuedMessage(id: string) {
		conversation.removeQueued(id);
	}

	function onWindowKeydown(e: KeyboardEvent) {
		if (!matchesShortcut(e, settingsStore.settings.shortcuts.stopResponse)) return;
		if (!chatStore.isStreamingFor(sessionId)) return;
		const target = e.target;
		if (target instanceof HTMLTextAreaElement || target instanceof HTMLInputElement) {
			if (target.selectionStart !== target.selectionEnd) return;
		}
		e.preventDefault();
		stop();
	}

	function revertModelSelection() {
		const session = sessionStore.sessions.find((item) => item.id === sessionId);
		if (session) modelStore.selectFromSession(session);
	}

	async function commitModelChange(option: ModelOption) {
		try {
			const updated = await updateSession(sessionId, {
				model_id: option.modelId,
				provider_id: option.providerId
			});
			sessionStore.updateSession(updated);
		} catch {
			revertModelSelection();
		}
	}

	async function onModelChange(option: ModelOption) {
		// Warn before switching to a provider that handles existing history
		// differently (e.g. Codex summarizes prior chain-of-thought). The model
		// store has already optimistically selected the option, so cancelling
		// must revert to the persisted session selection.
		const warnings = analyzeProviderSwitch(snapshotItems, option.providerMethod);
		if (warnings.length > 0) {
			pendingSwitch = { option, warnings };
			return;
		}
		await commitModelChange(option);
	}

	function confirmPendingSwitch() {
		const pending = pendingSwitch;
		pendingSwitch = null;
		if (pending) void commitModelChange(pending.option);
	}

	function cancelPendingSwitch() {
		pendingSwitch = null;
		revertModelSelection();
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
			<ChatThread
				{sessionId}
				{awaitingFirstAssistant}
				{firstTurnFlightDone}
				{firstTurnHandoffPending}
			/>
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
		onActiveChange={(active) => (firstTurnActive = active)}
		onPrepareFlight={() => {
			if (composerVariant === 'hero') heroFrameExiting = true;
			shellStore.dockComposer();
		}}
		onFlightDoneChange={(done) => {
			firstTurnFlightDone = done;
			firstTurnHandoffPending = !done;
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
				bind:this={composerRef}
				onSend={submit}
				onLocalUserMessage={showLocalUserMessage}
				onStop={stop}
				onRemoveQueued={removeQueuedMessage}
				{onModelChange}
				onWorkspaceChanged={() => chatStore.loadTranscript(sessionId)}
				onTranscriptCleared={() => {
					conversation.clearQueue();
					syncQueueState();
				}}
				{sessionId}
				disabled={connectionState.status !== 'ready'}
				streaming={chatStore.isStreamingFor(sessionId)}
				{queuedCount}
				{queuedMessages}
				waitingForReply={turnBusy || firstTurnActive}
				variant={composerVariant}
			/>
		</HeroComposerFrame>
	</div>
</div>

{#if pendingSwitch}
	<ProviderSwitchDialog
		providerName={pendingSwitch.option.providerName}
		warnings={pendingSwitch.warnings}
		onCancel={cancelPendingSwitch}
		onConfirm={confirmPendingSwitch}
	/>
{/if}

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
