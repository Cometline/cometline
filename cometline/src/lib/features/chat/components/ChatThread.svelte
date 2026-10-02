<script lang="ts">
	import { onDestroy } from 'svelte';
	import { fade } from 'svelte/transition';
	import { chatStore, type ChatItem } from '#lib/stores/chat.svelte.js';
	import { settingsStore } from '#lib/stores/settings.svelte.js';
	import { toolFoldLabel as formatToolFoldLabel } from '#lib/features/chat/thread-format.js';
	import type { AssistantStackContext } from '#lib/features/chat/assistant-stack-props.js';
	import FirstTurnAssistantSlot from '#lib/features/chat/components/FirstTurnAssistantSlot.svelte';
	import ChatThreadTurn from '#lib/features/chat/components/ChatThreadTurn.svelte';
	import JumpToBottom from '#lib/features/chat/components/JumpToBottom.svelte';
	import { buildThinkingAttribution } from '#lib/features/chat/thinking-attribution.js';
	import {
		selectFirstAssistantItem,
		showAssistantActivitySpinner,
		showAssistantRow as isAssistantRowVisible,
		type ThreadVisibilityContext
	} from '#lib/features/chat/thread-visibility.js';
	import { createFoldController } from '#lib/features/chat/thread-fold.svelte.js';
	import { createThreadScroll } from '#lib/features/chat/thread-scroll.svelte.js';
	import { createThreadVirtual } from '#lib/features/chat/thread-virtual.svelte.js';
	import { createThreadClocks } from '#lib/features/chat/thread-clocks.svelte.js';
	import { groupThreadItemsIntoTurns } from '#lib/features/chat/thread-turns.js';
	import type { ChatTurnPayload } from '#lib/actions/start-chat.js';
	import type { JobResource } from '#lib/client/cometmind.js';
	import {
		resolvePersona,
		personaAvatarSrcset as builtinAvatarSrcset
	} from '#lib/personas/index.js';
	import { personaAvatarCache } from '#lib/personas/avatar-cache.svelte.js';
	import SessionFindBar from '#lib/features/chat/components/SessionFindBar.svelte';
	import { createSessionFindController } from '#lib/features/chat/session-find.svelte.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';

	const TRANSCRIPT_IN = { duration: 140 };

	let {
		sessionId,
		awaitingFirstAssistant = false,
		firstTurnFlightDone = false,
		firstTurnHandoffPending = false,
		onNotifyAgent,
		onStartJob
	}: {
		sessionId: string;
		awaitingFirstAssistant?: boolean;
		firstTurnFlightDone?: boolean;
		firstTurnHandoffPending?: boolean;
		onNotifyAgent?: (payload: ChatTurnPayload) => void | Promise<void>;
		onStartJob?: (job: JobResource) => void | Promise<void>;
	} = $props();

	let resolvedPersona = $derived(
		resolvePersona(
			settingsStore.settings.app.personaId,
			settingsStore.settings.app.personas.custom
		)
	);
	let avatarSrc = $derived(personaAvatarCache.avatarSrcFor(resolvedPersona, 96));
	let avatarSrcset = $derived(
		resolvedPersona.kind === 'builtin' ? builtinAvatarSrcset(resolvedPersona) : undefined
	);

	let isSessionSynced = $derived(chatStore.sessionID === sessionId);
	let sessionStreaming = $derived(chatStore.isStreamingFor(sessionId));

	let snapshotItems = $state.raw<ChatItem[]>([]);

	$effect(() => {
		if (chatStore.sessionID !== sessionId) return;
		snapshotItems = chatStore.items;
	});

	let scrollerEl = $state<HTMLDivElement | undefined>(undefined);
	let threadItems = $derived(isSessionSynced ? chatStore.items : snapshotItems);
	let threadTurns = $derived(groupThreadItemsIntoTurns(threadItems));
	let searchableItemCount = $derived(
		threadItems.filter(
			(item) => (item.type === 'user' || item.type === 'assistant') && item.text.trim()
		).length
	);

	const sessionFind = createSessionFindController({
		getRoot: () => scrollerEl ?? null,
		getTurns: () => threadTurns,
		getSessionId: () => sessionId,
		getIsSessionSynced: () => isSessionSynced,
		getFindRequestId: () => shellStore.sessionFindRequestId,
		getSearchableItemCount: () => searchableItemCount
	});

	let thinkingForAssistant = $derived(buildThinkingAttribution(threadItems));
	// Prefer attributed tools/memory (hasVisibleThinkingBlock) so first-turn activity
	// pills mount live without waiting for text/reasoning — half-UI spinner otherwise.
	let firstAssistantItem = $derived(selectFirstAssistantItem(threadItems, thinkingForAssistant));
	let firstAssistantRowId = $derived(
		threadItems.find((item) => item.type === 'assistant')?.id ?? null
	);
	let streamingAssistantId = $derived.by(() => {
		if (!sessionStreaming) return null;
		const last = threadItems.at(-1);
		if (last?.type === 'user') return null;
		return threadItems.findLast((item) => item.type === 'assistant')?.id ?? null;
	});

	const fold = createFoldController({
		getSessionId: () => sessionId,
		getSessionEpoch: () => scroll.sessionEpoch,
		getIsSessionSynced: () => isSessionSynced,
		getItems: () => snapshotItems,
		getStreamingAssistantId: () => streamingAssistantId,
		getSessionStreaming: () => sessionStreaming
	});

	let firstUserId = $derived(threadItems.find((item) => item.type === 'user')?.id ?? null);
	let lastUserId = $derived(threadItems.findLast((item) => item.type === 'user')?.id ?? null);
	let userMessageCount = $derived(
		threadItems.reduce((count, item) => (item.type === 'user' ? count + 1 : count), 0)
	);

	function sessionHasCachedTranscript(targetSessionId: string) {
		if (chatStore.sessionID === targetSessionId && chatStore.items.length > 0) return true;
		if (
			chatStore.isStreamingFor(targetSessionId) ||
			chatStore.hasInFlightTurn(targetSessionId)
		) {
			return true;
		}
		return chatStore.getCachedItemCount(targetSessionId) > 0;
	}

	const scrollTopSink = {
		handler: (_top: number) => {}
	};

	const scroll = createThreadScroll({
		getSessionId: () => sessionId,
		getIsSessionSynced: () => isSessionSynced,
		getThreadItems: () => threadItems,
		getSessionStreaming: () => sessionStreaming,
		getLastUserId: () => lastUserId,
		getUserMessageCount: () => userMessageCount,
		getIsLoading: () => chatStore.isLoading,
		sessionHasCachedTranscript,
		getScroller: () => scrollerEl,
		getHasMoreHistory: () => chatStore.hasMoreHistory,
		getIsLoadingOlder: () => chatStore.isLoadingOlder,
		loadOlderTranscript: (id) => chatStore.loadOlderTranscript(id),
		onScrollTopChange: (top) => scrollTopSink.handler(top)
	});

	const virtual = createThreadVirtual({
		getSessionEpoch: () => scroll.sessionEpoch,
		getThreadTurns: () => threadTurns,
		getScroller: () => scrollerEl,
		getViewportHeight: () => scroll.viewportHeight,
		getActivePinnedUserId: () => scroll.activePinnedUserId,
		getActiveTurnMinHeight: () => scroll.activeTurnMinHeight,
		getLastUserId: () => lastUserId,
		getIsInitialTranscriptPaint: () => scroll.isInitialTranscriptPaint,
		getFindOpen: () => sessionFind.open,
		getFindActiveTurnIndex: () => sessionFind.activeTurnIndex,
		getFindJumpKey: () => `${sessionFind.query}:${sessionFind.activeIndex}`
	});
	scrollTopSink.handler = (top) => virtual.setScrollTop(top);

	onDestroy(() => sessionFind.closeFind({ restoreFocus: false }));

	function isMemoryInBuffer(item: Extract<ChatItem, { type: 'memory' }>) {
		return thinkingForAssistant.memoryIdsInBuffer.has(item.id);
	}

	const clocks = createThreadClocks({
		getThreadItems: () => threadItems,
		getSessionStreaming: () => sessionStreaming,
		getStreamingAssistantId: () => streamingAssistantId,
		hasStandaloneMemoryEvents: () =>
			threadItems.some((item) => item.type === 'memory' && !isMemoryInBuffer(item))
	});

	let heroGlowColor = $derived(settingsStore.settings.appearance.heroComposer.glowColor);

	let visibilityContext = $derived<ThreadVisibilityContext>({
		threadItems,
		thinkingForAssistant,
		streamingAssistantId,
		sessionStreaming,
		awaitingFirstAssistant,
		firstTurnFlightDone,
		firstTurnHandoffPending,
		firstUserId,
		firstAssistantRowId,
		firstAssistantItem
	});

	function showAssistantRow(item: Extract<ChatItem, { type: 'assistant' }>) {
		return isAssistantRowVisible(item, visibilityContext);
	}

	function showActivitySpinner(item: Extract<ChatItem, { type: 'assistant' }>) {
		return showAssistantActivitySpinner(item, streamingAssistantId, sessionStreaming);
	}

	let stackContext = $derived<AssistantStackContext>({
		threadItems,
		thinkingForAssistant,
		streamingAssistantId,
		sessionStreaming,
		sessionId,
		now: clocks.now,
		heroGlowColor,
		copiedId: clocks.copiedId,
		fold,
		toolFoldLabel: (item) =>
			formatToolFoldLabel(item, clocks.now, settingsStore.settings.cometmind.mcp.servers),
		onCopyMessage: clocks.copyMessage,
		onNotifyAgent,
		onStartJob
	});

	let showMessages = $derived(
		threadItems.length > 0 || (isSessionSynced && awaitingFirstAssistant && !firstUserId)
	);
	let transcriptFadeIn = $derived(
		awaitingFirstAssistant || scroll.isInitialTranscriptPaint ? { duration: 0 } : TRANSCRIPT_IN
	);

	function onThreadScroll() {
		scroll.onScroll();
	}
</script>

<div class="thread-wrap">
	<div
		class="thread scrollbar-none"
		bind:this={scrollerEl}
		onscroll={onThreadScroll}
		style:--thread-user-pin-offset-followup="{scroll.userPinScrollMargin}px"
		role="log"
		aria-label="Conversation"
		aria-live="polite"
	>
		<div class="thread-inner">
			{#if showMessages}
				<div
					class="thread-messages"
					class:hydrating={scroll.isInitialTranscriptPaint}
					in:fade={transcriptFadeIn}
				>
					{#if awaitingFirstAssistant && !firstUserId}
						<FirstTurnAssistantSlot
							{avatarSrc}
							{avatarSrcset}
							{firstTurnHandoffPending}
							{firstAssistantItem}
							{sessionStreaming}
							{stackContext}
							{showAssistantRow}
							{showActivitySpinner}
							flightPlaceholder
							ariaHidden
						/>
					{/if}

					{#if threadTurns.length > 0}
						<div
							class="thread-virtual"
							style:height="{virtual.virtualWindow.totalHeight}px"
						>
							{#each virtual.visibleTurns as entry (entry.item.id)}
								<ChatThreadTurn
									{entry}
									{avatarSrc}
									{avatarSrcset}
									{visibilityContext}
									{stackContext}
									{lastUserId}
									memoryCycleTick={clocks.memoryCycleTick}
									activePinnedUserId={scroll.activePinnedUserId}
									activeTurnMinHeight={scroll.activeTurnMinHeight}
									measureTurnHeight={virtual.measureTurnHeight}
									onTurnMeasured={virtual.onTurnMeasured}
								/>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
	{#if sessionFind.open}
		<SessionFindBar controller={sessionFind} />
	{/if}
	{#if scroll.showJumpToBottom}
		<JumpToBottom onclick={scroll.jumpToBottom} />
	{/if}
</div>

<style>
	.thread-wrap {
		position: absolute;
		inset: 0;
	}

	:global(::highlight(session-find-match)) {
		background: color-mix(in srgb, var(--color-facc15) 48%, transparent);
		color: inherit;
	}

	:global(::highlight(session-find-active)) {
		background: color-mix(in srgb, var(--color-f59e0b) 72%, transparent);
		color: inherit;
	}

	.thread {
		position: absolute;
		inset: 0;
		overflow-y: auto;
		overflow-x: hidden;
		overflow-anchor: none;
		padding: 32px var(--chat-gutter) var(--thread-padding-bottom);
	}

	.thread-inner {
		/* Fill the row after the avatar; thread-inner width already caps the measure. */
		--chat-content-column: calc(100% - var(--chat-avatar-size) - var(--chat-row-gap));
		--chat-assistant-column: var(--chat-content-column);
		width: 100%;
		max-width: var(--chat-thread-width);
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.thread-messages {
		display: flex;
		flex-direction: column;
		gap: 14px;
		width: 100%;
		transition: opacity var(--duration-session-switch) var(--ease-smooth);
	}

	.thread-virtual {
		position: relative;
		width: 100%;
	}

	.thread-messages.hydrating {
		opacity: 0;
		pointer-events: none;
	}

	@media (min-width: 768px) {
		.thread {
			padding: 40px var(--chat-gutter) var(--thread-padding-bottom);
		}

		.thread-inner {
			gap: 16px;
		}
	}

	@media (min-width: 1024px) {
		.thread {
			padding: 48px var(--chat-gutter) var(--thread-padding-bottom);
		}

		.thread-inner {
			gap: 18px;
		}
	}

	@media (min-width: 1280px) {
		.thread {
			padding: 56px var(--chat-gutter) var(--thread-padding-bottom);
		}
	}
</style>
