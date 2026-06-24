<script lang="ts">
	import ThinkingBlock from '$lib/components/chat/ThinkingBlock.svelte';
	import MemoryCard from '$lib/components/chat/MemoryCard.svelte';
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import SubagentPanel from '$lib/components/chat/SubagentPanel.svelte';
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
{:else}
	<SubagentPanel
		item={entry.subagent}
		expanded={ctx.fold.subagentExpanded(entry.subagent.id)}
		{nested}
		{toggleDisabled}
		onToggle={() => ctx.fold.toggleSubagent(entry.subagent.id)}
	/>
{/if}
