<script lang="ts">
	import ThinkingBlock from '$lib/components/chat/ThinkingBlock.svelte';
	import MemoryCard from '$lib/components/chat/MemoryCard.svelte';
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import SubagentPanel from '$lib/components/chat/SubagentPanel.svelte';
	import { TriangleAlert } from '@lucide/svelte';
	import { getChatTurnContext } from '$lib/conversation/chat-turn-context';
	import { isTimelineEntryToggleDisabled } from '$lib/conversation/thinking-attribution';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import type { TimelineEntry } from '$lib/conversation/thinking-attribution';

	let {
		entry,
		assistant,
		assistantId,
		nested = false,
		showThinkingSpinner = false,
		cycling = false
	}: {
		entry: TimelineEntry;
		assistant: Extract<ChatItem, { type: 'assistant' }>;
		assistantId: string;
		nested?: boolean;
		showThinkingSpinner?: boolean;
		cycling?: boolean;
	} = $props();

	const ctx = $derived(getChatTurnContext());
	const toggleDisabled = $derived(isTimelineEntryToggleDisabled(entry));

	function thinkingActive(pending?: boolean) {
		return pending === true;
	}

	function segmentKey(entry: Extract<TimelineEntry, { kind: 'reasoning' }>) {
		return `${assistantId}-seg-${entry.segmentIndex}`;
	}
</script>

{#if entry.kind === 'reasoning'}
	{@const key = segmentKey(entry)}
	<ThinkingBlock
		text={entry.text}
		pending={entry.pending}
		expanded={ctx.fold.thinkingExpanded(assistant, key, entry.segmentIndex, entry.pending)}
		showSpinner={thinkingActive(entry.pending) && showThinkingSpinner}
		{nested}
		{toggleDisabled}
		onToggle={() => ctx.fold.toggleThinking(assistant, key, entry.segmentIndex, entry.pending)}
	/>
{:else if entry.kind === 'memory'}
	{@const memoryKey = `${assistantId}-memory`}
	<MemoryCard
		memories={entry.memories}
		expanded={ctx.fold.memoryInThinkingExpanded(memoryKey)}
		{nested}
		onToggle={() => ctx.fold.toggleMemoryInThinking(memoryKey)}
		{cycling}
	/>
{:else if entry.kind === 'tool'}
	<ToolFoldPanel
		item={entry.tool}
		label={ctx.toolFoldLabel(entry.tool)}
		expanded={ctx.fold.toolOutputExpanded(entry.tool)}
		{nested}
		{toggleDisabled}
		onToggle={() => ctx.fold.toggleToolOutput(entry.tool.id)}
		sessionId={ctx.sessionId}
		onNotifyAgent={ctx.onNotifyAgent}
		onStartJob={ctx.onStartJob}
	/>
{:else if entry.kind === 'subagent'}
	<SubagentPanel
		item={entry.subagent}
		expanded={ctx.fold.subagentExpanded(entry.subagent.id)}
		{nested}
		{toggleDisabled}
		onToggle={() => ctx.fold.toggleSubagent(entry.subagent.id)}
	/>
{:else}
	<div class="activity-error" class:nested>
		<div class="activity-error-title"><TriangleAlert size={13} /><span>Error</span></div>
		<p>{entry.error.text}</p>
	</div>
{/if}

<style>
	.activity-error {
		box-sizing: border-box;
		width: min(var(--assistant-activity-width, 80%), 100%);
		min-width: 0;
		border: 1px solid var(--status-error-border);
		background: var(--status-error-bg);
		border-radius: 12px;
		padding: 10px 12px;
		color: var(--status-error);
	}

	.activity-error.nested {
		width: 100%;
	}

	.activity-error-title {
		display: flex;
		align-items: center;
		gap: 7px;
		margin-bottom: 6px;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
	}

	.activity-error p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
		overflow-wrap: break-word;
	}
</style>
