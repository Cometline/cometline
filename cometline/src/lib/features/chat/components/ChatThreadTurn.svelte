<script lang="ts">
	import type { Action } from 'svelte/action';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import { usageText } from '$lib/features/chat/thread-format';
	import { pinnedJobProposalToolIds } from '$lib/features/chat/thinking-attribution';
	import type { AssistantStackContext } from '$lib/features/chat/assistant-stack-props';
	import {
		hideAssistantAvatarForFirstTurn,
		showAssistantActivitySpinner,
		showAssistantRow as isAssistantRowVisible,
		showFirstTurnAvatarSlot,
		firstAssistantInNormalList as shouldShowAssistantInNormalList,
		type ThreadVisibilityContext
	} from '$lib/features/chat/thread-visibility';
	import { startsSpeakerRun } from '$lib/features/chat/thread-view-helpers';
	import type { VisibleTurnEntry } from '$lib/features/chat/thread-virtual.svelte';
	import FirstTurnAssistantSlot from '$lib/features/chat/components/FirstTurnAssistantSlot.svelte';
	import UserMessageRow from '$lib/features/chat/components/UserMessageRow.svelte';
	import MemoryEventRow from '$lib/features/chat/components/MemoryEventRow.svelte';
	import AssistantMessageRow from '$lib/features/chat/components/AssistantMessageRow.svelte';
	import ToolMessageRow from '$lib/features/chat/components/ToolMessageRow.svelte';
	import SubagentMessageRow from '$lib/features/chat/components/SubagentMessageRow.svelte';
	import ErrorEventRow from '$lib/features/chat/components/ErrorEventRow.svelte';

	let {
		entry,
		avatarSrc = '',
		avatarSrcset = '',
		visibilityContext,
		stackContext,
		lastUserId,
		memoryCycleTick,
		activePinnedUserId,
		activeTurnMinHeight,
		measureTurnHeight,
		onTurnMeasured
	}: {
		entry: VisibleTurnEntry;
		avatarSrc?: string;
		avatarSrcset?: string;
		visibilityContext: ThreadVisibilityContext;
		stackContext: AssistantStackContext;
		lastUserId: string | null;
		memoryCycleTick: number;
		activePinnedUserId: string | null;
		activeTurnMinHeight: number;
		measureTurnHeight: Action<
			HTMLElement,
			{ id: string; onMeasure: (id: string, height: number) => void }
		>;
		onTurnMeasured: (id: string, height: number) => void;
	} = $props();

	const turn = $derived(entry.item);
	const isActiveTurn = $derived(activePinnedUserId === turn.id);
	const thinking = $derived(visibilityContext.thinkingForAssistant);
	const firstAssistantId = $derived(visibilityContext.firstAssistantItem?.id ?? null);
	const embeddedPinnedJobIds = $derived(pinnedJobProposalToolIds(visibilityContext.threadItems));

	function showAssistantRow(item: Extract<ChatItem, { type: 'assistant' }>) {
		return isAssistantRowVisible(item, visibilityContext);
	}

	function showActivitySpinner(item: Extract<ChatItem, { type: 'assistant' }>) {
		return showAssistantActivitySpinner(
			item,
			visibilityContext.streamingAssistantId,
			visibilityContext.sessionStreaming
		);
	}
</script>

<div
	class="thread-turn"
	class:thread-turn-active={isActiveTurn}
	data-turn-id={turn.id}
	style:top="{entry.offset}px"
	style:min-height={isActiveTurn ? `${activeTurnMinHeight}px` : undefined}
	use:measureTurnHeight={{ id: turn.id, onMeasure: onTurnMeasured }}
>
	{#if turn.user}
		<UserMessageRow
			item={turn.user}
			{avatarSrc}
			{avatarSrcset}
			continuationRow={!startsSpeakerRun(
				visibilityContext.threadItems,
				turn.userIndex,
				'user'
			)}
			copiedId={stackContext.copiedId}
			onCopyMessage={stackContext.onCopyMessage}
			flyOnReveal={turn.user.id !== visibilityContext.firstUserId}
		/>
		{#if showFirstTurnAvatarSlot(visibilityContext) && turn.user.id === visibilityContext.firstUserId}
			<FirstTurnAssistantSlot
				{avatarSrc}
				{avatarSrcset}
				firstTurnHandoffPending={visibilityContext.firstTurnHandoffPending}
				firstAssistantItem={visibilityContext.firstAssistantItem}
				sessionStreaming={visibilityContext.sessionStreaming}
				{stackContext}
				{showAssistantRow}
				{showActivitySpinner}
				flightPlaceholder={!firstAssistantId}
				ariaHidden={!firstAssistantId}
			/>
		{/if}
	{/if}
	{#each turn.items as { item, index } (item.id)}
		{#if item.type === 'assistant' && showAssistantRow(item) && shouldShowAssistantInNormalList(item, visibilityContext)}
			<AssistantMessageRow
				{item}
				threadItems={visibilityContext.threadItems}
				{index}
				{avatarSrc}
				{avatarSrcset}
				{stackContext}
				{showActivitySpinner}
				hideAvatarForFirstTurn={hideAssistantAvatarForFirstTurn(
					item,
					visibilityContext.firstTurnHandoffPending,
					visibilityContext.firstAssistantRowId
				)}
				deferMarkdown={entry.skipHydrationMarkdown &&
					item.id !== visibilityContext.streamingAssistantId}
			/>
		{:else if item.type === 'tool' && !thinking.toolIdsInBuffer.has(item.id) && !embeddedPinnedJobIds.has(item.id)}
			<ToolMessageRow
				{item}
				threadItems={visibilityContext.threadItems}
				{index}
				{avatarSrc}
				{avatarSrcset}
				sessionId={stackContext.sessionId}
				toolFoldLabel={stackContext.toolFoldLabel}
				fold={stackContext.fold}
				onNotifyAgent={stackContext.onNotifyAgent}
				onStartJob={stackContext.onStartJob}
			/>
		{:else if item.type === 'subagent' && !thinking.subagentIdsInBuffer.has(item.id)}
			<SubagentMessageRow
				{item}
				threadItems={visibilityContext.threadItems}
				{index}
				{avatarSrc}
				{avatarSrcset}
				fold={stackContext.fold}
			/>
		{:else if item.type === 'memory' && !thinking.memoryIdsInBuffer.has(item.id)}
			<MemoryEventRow {item} {memoryCycleTick} />
		{:else if item.type === 'status'}
			<div class="status">{usageText(item)}</div>
		{:else if item.type === 'error' && !thinking.errorIdsInBuffer.has(item.id)}
			<ErrorEventRow {item} />
		{/if}
	{/each}
	{#if turn.id === lastUserId}
		<div class="thread-latest-sentinel" data-thread-latest-sentinel aria-hidden="true"></div>
	{/if}
</div>

<style>
	.thread-turn {
		position: absolute;
		left: 0;
		right: 0;
		display: flex;
		flex-direction: column;
		gap: 14px;
		/* Keep min-height growth from sticky-anchoring the bubble (esp. mini). */
		overflow-anchor: none;
		/*
		 * Reintroduce release-style paint deferral for overscan mounts without a
		 * plaintext↔{@html} swap: browser skips layout/paint until near viewport.
		 * JS/Shiki still only skipped during hydration (B); this is paint defense (C).
		 */
		content-visibility: auto;
		contain-intrinsic-block-size: auto 500px;
	}

	.thread-turn-active :global(.user-row) {
		scroll-margin-top: var(--thread-user-pin-offset-followup);
		overflow-anchor: none;
	}

	.thread-latest-sentinel {
		width: 1px;
		height: 1px;
		pointer-events: none;
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
