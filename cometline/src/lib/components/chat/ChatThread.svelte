<script lang="ts">
	import { fade } from 'svelte/transition';
	import { chatStore, type ChatItem } from '$lib/stores/chat.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { chatDebug, chatDebugEnabled, summarizeChatItem } from '$lib/debug/chat';
	import {
		toolFoldLabel as formatToolFoldLabel,
		usageText
	} from '$lib/conversation/thread-format';
	import type { AssistantStackContext } from '$lib/conversation/assistant-stack-props';
	import FirstTurnAssistantSlot from '$lib/components/chat/FirstTurnAssistantSlot.svelte';
	import UserMessageRow from '$lib/components/chat/UserMessageRow.svelte';
	import MemoryEventRow from '$lib/components/chat/MemoryEventRow.svelte';
	import AssistantMessageRow from '$lib/components/chat/AssistantMessageRow.svelte';
	import ToolMessageRow from '$lib/components/chat/ToolMessageRow.svelte';
	import SubagentMessageRow from '$lib/components/chat/SubagentMessageRow.svelte';
	import ErrorEventRow from '$lib/components/chat/ErrorEventRow.svelte';
	import JumpToBottom from '$lib/components/chat/JumpToBottom.svelte';
	import {
		buildThinkingAttribution,
		pinnedJobProposalToolIds
	} from '$lib/conversation/thinking-attribution';
	import { startsSpeakerRun } from '$lib/conversation/thread-view-helpers';
	import {
		firstAssistantInNormalList as shouldShowAssistantInNormalList,
		hideAssistantAvatarForFirstTurn,
		showAssistantActivitySpinner,
		showAssistantRow as isAssistantRowVisible,
		showFirstTurnAvatarSlot,
		type ThreadVisibilityContext
	} from '$lib/conversation/thread-visibility';
	import { createFoldController } from '$lib/conversation/thread-fold.svelte';
	import { createThreadScroll } from '$lib/conversation/thread-scroll.svelte';
	import { createThreadClocks } from '$lib/conversation/thread-clocks.svelte';
	import { hasReasoning } from '$lib/conversation/reasoning';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import type { JobResource } from '$lib/client/cometmind';
	import { resolvePersona, personaAvatarSrcset as builtinAvatarSrcset } from '$lib/personas';
	import { personaAvatarCache } from '$lib/personas/avatar-cache.svelte';

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
		resolvePersona(settingsStore.settings.app.personaId, settingsStore.settings.app.personas.custom)
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
	let embeddedPinnedJobIds = $derived(pinnedJobProposalToolIds(threadItems));
	let firstAssistantItem = $derived(
		threadItems.find(
			(item) => item.type === 'assistant' && (item.text.trim() || hasReasoning(item))
		) as Extract<ChatItem, { type: 'assistant' }> | undefined
	);
	let firstAssistantId = $derived(firstAssistantItem?.id ?? null);
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

	const scroll = createThreadScroll({
		getSessionId: () => sessionId,
		getIsSessionSynced: () => isSessionSynced,
		getThreadItems: () => threadItems,
		getSessionStreaming: () => sessionStreaming,
		getLastUserId: () => lastUserId,
		getUserMessageCount: () => userMessageCount,
		getIsLoading: () => chatStore.isLoading,
		sessionHasCachedTranscript
	});

	$effect(() => {
		scroll.setScroller(scrollerEl);
	});

	let thinkingForAssistant = $derived(buildThinkingAttribution(threadItems));

	function isToolInBuffer(item: Extract<ChatItem, { type: 'tool' }>) {
		return thinkingForAssistant.toolIdsInBuffer.has(item.id);
	}

	function isSubagentInBuffer(item: Extract<ChatItem, { type: 'subagent' }>) {
		return thinkingForAssistant.subagentIdsInBuffer.has(item.id);
	}

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
		toolFoldLabel: (item) => formatToolFoldLabel(item, clocks.now),
		onCopyMessage: clocks.copyMessage,
		onNotifyAgent,
		onStartJob
	});

	let bottomSpacerHeight = $derived(
		userMessageCount > 1 && scroll.viewportHeight > 0 && threadItems.at(-1)?.type === 'user'
			? Math.max(0, scroll.viewportHeight - 240)
			: 0
	);
	let showMessages = $derived(
		threadItems.length > 0 || (isSessionSynced && awaitingFirstAssistant && !firstUserId)
	);
	let transcriptFadeIn = $derived(
		awaitingFirstAssistant || scroll.isInitialTranscriptPaint ? { duration: 0 } : TRANSCRIPT_IN
	);

	let renderDebugSnapshot = $derived.by(() => ({
		awaitingFirstAssistant,
		firstTurnFlightDone,
		firstTurnHandoffPending,
		isStreaming: sessionStreaming,
		firstUserId,
		firstAssistantId,
		firstAssistantRowId,
		firstAssistantItem: firstAssistantItem ? summarizeChatItem(firstAssistantItem) : null,
		firstAssistantVisible: firstAssistantItem ? showAssistantRow(firstAssistantItem) : false,
		items: threadItems.map((item, index) => {
			if (item.type !== 'assistant') return { index, ...summarizeChatItem(item) };
			return {
				index,
				...summarizeChatItem(item),
				showAssistantRow: showAssistantRow(item),
				isFirstAssistant: item.id === firstAssistantId,
				renderedInFirstTurnSlot: awaitingFirstAssistant && item.id === firstAssistantId,
				excludedFromNormalList: awaitingFirstAssistant && item.id === firstAssistantId,
				showAssistantPending: isAssistantRowVisible(item, visibilityContext)
			};
		})
	}));

	$inspect(renderDebugSnapshot).with((type, snapshot) => {
		if (!chatDebugEnabled()) return;
		chatDebug('thread:$inspect', { type, ...snapshot });
	});
</script>

<div class="thread-wrap">
	<div
		class="thread scrollbar-none"
		bind:this={scrollerEl}
		onscroll={scroll.onScroll}
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

					{#each threadItems as item, index (item.id)}
						{#if item.type === 'user'}
							<UserMessageRow
								{item}
								{avatarSrc}
								{avatarSrcset}
								continuationRow={!startsSpeakerRun(threadItems, index, 'user')}
								copiedId={clocks.copiedId}
								onCopyMessage={clocks.copyMessage}
							/>
							{#if showFirstTurnAvatarSlot(visibilityContext) && item.id === firstUserId}
								<FirstTurnAssistantSlot
									{avatarSrc}
									{avatarSrcset}
									{firstTurnHandoffPending}
									{firstAssistantItem}
									{sessionStreaming}
									{stackContext}
									{showAssistantRow}
									{showActivitySpinner}
									flightPlaceholder={!firstAssistantId}
									ariaHidden={!firstAssistantId}
								/>
							{/if}
						{:else if item.type === 'assistant' && showAssistantRow(item) && shouldShowAssistantInNormalList(item, visibilityContext)}
							<AssistantMessageRow
								{item}
								{threadItems}
								{index}
								{avatarSrc}
								{avatarSrcset}
								{stackContext}
								{showActivitySpinner}
								hideAvatarForFirstTurn={hideAssistantAvatarForFirstTurn(
									item,
									firstTurnHandoffPending,
									firstAssistantRowId
								)}
							/>
						{:else if item.type === 'tool' && !isToolInBuffer(item) && !embeddedPinnedJobIds.has(item.id)}
							<ToolMessageRow
								{item}
								{threadItems}
								{index}
								{avatarSrc}
								{avatarSrcset}
								{sessionId}
								toolFoldLabel={stackContext.toolFoldLabel}
								{fold}
								{onNotifyAgent}
								{onStartJob}
							/>
						{:else if item.type === 'subagent' && !isSubagentInBuffer(item)}
							<SubagentMessageRow
							{item}
							{threadItems}
							{index}
							{avatarSrc}
							{avatarSrcset}
							{fold}
						/>
						{:else if item.type === 'memory' && !isMemoryInBuffer(item)}
							<MemoryEventRow {item} memoryCycleTick={clocks.memoryCycleTick} />
						{:else if item.type === 'status'}
							<div class="status">{usageText(item)}</div>
						{:else if item.type === 'error'}
							<ErrorEventRow {item} />
						{/if}
					{/each}
					{#if bottomSpacerHeight > 0}
						<div
							class="thread-bottom-spacer"
							style:height="{bottomSpacerHeight}px"
							aria-hidden="true"
						></div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
	{#if scroll.showJumpToBottom}
		<JumpToBottom onclick={scroll.jumpToBottom} />
	{/if}
</div>

<style>
	.thread-wrap {
		position: absolute;
		inset: 0;
	}

	.thread {
		position: absolute;
		inset: 0;
		overflow-y: auto;
		overflow-x: hidden;
		padding: 32px var(--chat-gutter) var(--thread-padding-bottom);
	}

	.thread-inner {
		--chat-content-column: min(
			var(--chat-content-max),
			calc(100% - var(--chat-avatar-size) - var(--chat-row-gap))
		);
		--chat-assistant-column: var(--chat-content-column);
		width: min(var(--chat-thread-width), 100%);
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.thread-messages {
		display: flex;
		flex-direction: column;
		gap: 14px;
		transition: opacity var(--duration-session-switch) var(--ease-smooth);
	}

	.thread-messages.hydrating {
		opacity: 0;
		pointer-events: none;
	}

	.thread-bottom-spacer {
		flex: 0 0 auto;
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

	.status {
		align-self: center;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 6px 10px;
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.74);
		border: 1px solid var(--border-soft);
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
