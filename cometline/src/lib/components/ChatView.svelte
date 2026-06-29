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
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import type { ChatItem } from '$lib/types';
	import type { ModelOption } from '$lib/stores/model.svelte';
	import ProviderSwitchDialog from '$lib/components/ProviderSwitchDialog.svelte';
	import { analyzeProviderSwitch, type ProviderSwitchWarning } from '$lib/provider-switch';
	import { startJobInSession } from '$lib/jobs/start-job-in-chat';
	import type { JobResource } from '$lib/client/cometmind';
	import { createChatViewController } from '$lib/conversation/chat-view-controller.svelte';

	const THREAD_IN = { duration: 140 };

	let {
		sessionId,
		bootMessage = '',
		compact = false
	}: { sessionId: string; bootMessage?: string; compact?: boolean } = $props();

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
				if (compact && firstTurn) {
					awaitingFirstAssistant = true;
					firstTurnFlightDone = true;
					firstTurnHandoffPending = false;
					stageUser(payload.text, payload.images);
					revealStagedUser();
					return;
				}
				if (firstTurn) {
					awaitingFirstAssistant = true;
					firstTurnFlightDone = false;
					firstTurnHandoffPending = true;
					if (!firstTurnFlight) {
						firstTurnFlightDone = true;
						firstTurnHandoffPending = false;
						return;
					}
					return firstTurnFlight
						?.runAsync(payload.text, payload.images, {
							stageUser,
							revealStagedUser
						})
						.catch((error) => {
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

	const chatView = createChatViewController({
		getSessionId: () => sessionId,
		getHasVisibleConversation: () => hasVisibleConversation,
		getFirstTurnActive: () => firstTurnActive,
		getFirstTurnFlightDone: () => firstTurnFlightDone,
		getAwaitingFirstAssistant: () => awaitingFirstAssistant,
		getForceDocked: () => compact,
		enqueue: (payload) => {
			void conversation.enqueue(payload);
		},
		cancelTurn: () => conversation.cancel()
	});

	let composerVariant = $derived(chatView.composerVariant);
	let heroLayout = $derived(chatView.heroLayout);
	let composerFocusRequestId = $derived(shellStore.composerFocusRequestId);

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
		if (shellStore.focusedPane !== 'chat') return;
		composerRef?.focus();
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
		if (!composerFocusRequestId || shellStore.focusedPane !== 'chat') return;
		composerRef?.focus();
	});

	$effect(() => {
		if (!hasVisibleConversation && !firstTurnActive && !awaitingFirstAssistant) {
			firstTurnFlightDone = false;
			heroFrameExiting = false;
		}
	});

	function submit(payload: ChatTurnPayload | string) {
		chatView.submit(payload);
	}

	function startJobFromCard(job: JobResource) {
		return startJobInSession(job, sessionId, submit);
	}

	function showLocalUserMessage(text: string) {
		chatStore.appendLocalUserMessage(sessionId, text);
	}

	function stop() {
		chatView.stop();
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

	function onWindowFocus() {
		if (!compact) return;
		if (shellStore.focusedPane !== 'chat') return;
		composerRef?.focus();
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

	async function openInMainWindow() {
		if (!sessionId) return;
		await window.electronAPI?.openSessionInMainWindow?.(sessionId);
	}
</script>

<svelte:window onkeydown={onWindowKeydown} onfocus={onWindowFocus} />

<div
	class="chat-home"
	class:hero-layout={heroLayout}
	class:first-turn-active={firstTurnActive}
	class:compact
	bind:this={chatHome}
>
	{#if compact}
		<div class="mini-titlebar" aria-label="Mini window drag area">
			<span>Mini Chat</span>
			<button
				class="mini-open-main"
				type="button"
				title="Open this chat in the main window"
				aria-label="Open this chat in the main window"
				onclick={openInMainWindow}
			>
				<svg viewBox="0 0 16 16" aria-hidden="true">
					<path d="M5 3.5h7.5V11" />
					<path d="M12.5 3.5 6.25 9.75" />
					<path d="M10.5 12.5h-7v-7" />
				</svg>
			</button>
		</div>
	{/if}

	{#if !compact && !hasVisibleConversation && !firstTurnActive}
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
				onNotifyAgent={submit}
				onStartJob={startJobFromCard}
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
		class:centered={!compact && shellStore.composerPhase === 'centered'}
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
				disabled={!chatView.canSend}
				streaming={chatStore.isStreamingFor(sessionId)}
				{queuedCount}
				{queuedMessages}
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

	.chat-home.compact {
		flex: none;
		height: 100vh;
		min-height: 100vh;
		--mini-titlebar-height: 46px;
		background:
			radial-gradient(
				circle at top,
				color-mix(in srgb, var(--hero-composer-glow-color) 16%, transparent),
				transparent 42%
			),
			var(--app-bg);
	}

	.mini-titlebar {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: var(--mini-titlebar-height);
		z-index: 40;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 0 96px;
		border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
		background: color-mix(in srgb, var(--panel-bg) 82%, transparent);
		color: var(--text-muted);
		font-size: 11px;
		font-weight: 650;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		user-select: none;
		-webkit-app-region: drag;
	}

	.mini-open-main {
		position: absolute;
		right: 12px;
		top: 50%;
		transform: translateY(-50%);
		width: 28px;
		height: 28px;
		display: grid;
		place-items: center;
		padding: 0;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 999px;
		background: color-mix(in srgb, var(--panel-bg) 88%, var(--text-main) 6%);
		color: var(--text-main);
		cursor: pointer;
		-webkit-app-region: no-drag;
	}

	.mini-open-main svg {
		width: 14px;
		height: 14px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.7;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.mini-open-main:hover {
		border-color: color-mix(in srgb, var(--hero-composer-glow-color) 54%, var(--border-soft));
		background: color-mix(in srgb, var(--hero-composer-glow-color) 18%, var(--panel-bg));
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

	.chat-home.compact .thread-shell.docked {
		top: var(--mini-titlebar-height);
		bottom: calc(var(--thread-dock-inset) - 18px);
	}

	.boot-error {
		margin: 18px 0 0;
		max-width: 520px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--status-error);
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

	.chat-home.compact .composer-wrapper {
		padding-inline: 14px;
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
