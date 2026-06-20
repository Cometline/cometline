<script lang="ts">
	import { slide } from 'svelte/transition';
	import {
		Brain,
		ChevronDown,
		CircleCheck,
		CircleX,
		LoaderCircle,
		Terminal,
		TriangleAlert
	} from '@lucide/svelte';
	import ThinkingSpinner from '$lib/components/ThinkingSpinner.svelte';
	import ThinkingBlock from '$lib/components/chat/ThinkingBlock.svelte';
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import SubagentPanel from '$lib/components/chat/SubagentPanel.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import type { TimelineEntry, InjectedMemory } from '$lib/conversation/thinking-attribution';
	import { subagentProgressLabel } from '$lib/conversation/subagent-display';

	const FOLD_IN = { duration: 180 };

	let {
		assistantId,
		timeline,
		parentExpanded,
		onToggleParent,
		timelineEntryKey,
		toolFoldLabel,
		thinkingExpanded,
		toggleThinking,
		memoryInThinkingExpanded,
		toggleMemoryInThinking,
		thinkingActive,
		showThinkingSpinner,
		toolOutputExpanded,
		toggleToolOutput,
		subagentExpanded,
		toggleSubagent
	}: {
		assistantId: string;
		timeline: TimelineEntry[];
		parentExpanded: boolean;
		onToggleParent: () => void;
		timelineEntryKey: (entry: TimelineEntry) => string;
		toolFoldLabel: (item: Extract<ChatItem, { type: 'tool' }>) => string;
		thinkingExpanded: (segmentKey: string, pending?: boolean) => boolean;
		toggleThinking: (segmentKey: string, pending?: boolean) => void;
		memoryInThinkingExpanded: (segmentKey: string) => boolean;
		toggleMemoryInThinking: (segmentKey: string) => void;
		thinkingActive: (pending?: boolean) => boolean;
		showThinkingSpinner: boolean;
		toolOutputExpanded: (item: Extract<ChatItem, { type: 'tool' }>) => boolean;
		toggleToolOutput: (id: string) => void;
		subagentExpanded: (id: string) => boolean;
		toggleSubagent: (id: string) => void;
	} = $props();

	let firstEntry = $derived(timeline[0]);
	let childEntries = $derived(timeline.slice(1));

	function thinkingLabel(memories?: InjectedMemory[]) {
		if (!memories?.length) return 'Thinking';
		return `Thinking · ${memories.length} memor${memories.length === 1 ? 'y' : 'ies'}`;
	}

	function parentLabel(entry: TimelineEntry) {
		if (entry.kind === 'reasoning') return thinkingLabel(entry.memories);
		if (entry.kind === 'tool') return toolFoldLabel(entry.tool);
		return subagentProgressLabel(entry.subagent);
	}

	function segmentKey(entry: Extract<TimelineEntry, { kind: 'reasoning' }>) {
		return `${assistantId}-seg-${entry.segmentIndex}`;
	}
</script>

{#if firstEntry}
	<div class="fold-panel activity-group">
		<button
			type="button"
			class="fold-toggle activity-group-toggle"
			aria-expanded={parentExpanded}
			onclick={onToggleParent}
		>
			{#if firstEntry.kind === 'reasoning'}
				<Brain size={13} />
			{:else}
				<Terminal size={13} />
			{/if}
			<span>{parentLabel(firstEntry)}</span>
			{#if firstEntry.kind === 'reasoning' && showThinkingSpinner && thinkingActive(firstEntry.pending)}
				<ThinkingSpinner size={12} label="Thinking" />
			{:else if firstEntry.kind === 'tool'}
				{#if firstEntry.tool.pending}
					<LoaderCircle size={12} class="spin" />
				{:else if firstEntry.tool.error}
					<TriangleAlert size={12} />
				{:else}
					<CircleCheck size={12} />
				{/if}
			{:else if firstEntry.kind === 'subagent'}
				{#if firstEntry.subagent.pending}
					<LoaderCircle size={12} class="spin" />
				{:else if firstEntry.subagent.status === 'failed' || firstEntry.subagent.status === 'incomplete'}
					<TriangleAlert size={12} />
				{:else if firstEntry.subagent.status === 'cancelled'}
					<CircleX size={12} />
				{:else}
					<CircleCheck size={12} />
				{/if}
			{/if}
			<ChevronDown size={13} class={parentExpanded ? 'expanded' : ''} />
		</button>
		{#if parentExpanded}
			<div class="fold-body activity-group-body" transition:slide={FOLD_IN}>
				{#if firstEntry.kind === 'reasoning'}
					{@const key = segmentKey(firstEntry)}
					<ThinkingBlock
						text={firstEntry.text}
						pending={firstEntry.pending}
						memories={firstEntry.memories}
						expanded={true}
						memoryExpanded={memoryInThinkingExpanded(key)}
						showSpinner={false}
						contentOnly={true}
						onToggle={() => toggleThinking(key, firstEntry.pending)}
						onToggleMemory={() => toggleMemoryInThinking(key)}
					/>
				{:else if firstEntry.kind === 'tool'}
					<ToolFoldPanel
						item={firstEntry.tool}
						label={toolFoldLabel(firstEntry.tool)}
						expanded={toolOutputExpanded(firstEntry.tool)}
						contentOnly={true}
						onToggle={() => toggleToolOutput(firstEntry.tool.id)}
					/>
				{:else}
					<SubagentPanel
						item={firstEntry.subagent}
						expanded={subagentExpanded(firstEntry.subagent.id)}
						contentOnly={true}
						onToggle={() => toggleSubagent(firstEntry.subagent.id)}
					/>
				{/if}
				{#each childEntries as entry (timelineEntryKey(entry))}
					{#if entry.kind === 'reasoning'}
						{@const key = segmentKey(entry)}
						<ThinkingBlock
							text={entry.text}
							pending={entry.pending}
							memories={entry.memories}
							expanded={thinkingExpanded(key, entry.pending)}
							memoryExpanded={memoryInThinkingExpanded(key)}
							showSpinner={thinkingActive(entry.pending) && showThinkingSpinner}
							nested={true}
							onToggle={() => toggleThinking(key, entry.pending)}
							onToggleMemory={() => toggleMemoryInThinking(key)}
						/>
					{:else if entry.kind === 'tool'}
						<ToolFoldPanel
							item={entry.tool}
							label={toolFoldLabel(entry.tool)}
							expanded={toolOutputExpanded(entry.tool)}
							nested={true}
							onToggle={() => toggleToolOutput(entry.tool.id)}
						/>
					{:else}
						<SubagentPanel
							item={entry.subagent}
							expanded={subagentExpanded(entry.subagent.id)}
							nested={true}
							onToggle={() => toggleSubagent(entry.subagent.id)}
						/>
					{/if}
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	.fold-panel {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.fold-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		align-self: flex-start;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.72);
		color: var(--text-muted);
		border-radius: 999px;
		padding: 5px 10px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}

	.fold-toggle:hover {
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-main);
	}

	.fold-toggle :global(svg.expanded) {
		transform: rotate(180deg);
	}

	.fold-body {
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.68);
		border-radius: 12px;
		padding: 10px 12px;
		color: var(--text-muted);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.04);
	}

	.activity-group-body {
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-self: stretch;
	}
</style>
