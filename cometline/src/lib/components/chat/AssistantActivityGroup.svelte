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
	import MemoryCard from '$lib/components/chat/MemoryCard.svelte';
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import SubagentPanel from '$lib/components/chat/SubagentPanel.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import type { TimelineEntry, InjectedMemory } from '$lib/conversation/thinking-attribution';
	import { subagentProgressLabel } from '$lib/conversation/subagent-display';

	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import type { JobResource } from '$lib/client/cometmind';

	const FOLD_IN = { duration: 180 };

	let {
		assistant,
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
		toggleSubagent,
		sessionId = '',
		onNotifyAgent,
		onStartJob
	}: {
		assistant: Extract<ChatItem, { type: 'assistant' }>;
		assistantId: string;
		timeline: TimelineEntry[];
		parentExpanded: boolean;
		onToggleParent: () => void;
		timelineEntryKey: (entry: TimelineEntry) => string;
		toolFoldLabel: (item: Extract<ChatItem, { type: 'tool' }>) => string;
		thinkingExpanded: (
			assistant: Extract<ChatItem, { type: 'assistant' }>,
			segmentKey: string,
			segmentIndex: number,
			pending?: boolean
		) => boolean;
		toggleThinking: (
			assistant: Extract<ChatItem, { type: 'assistant' }>,
			segmentKey: string,
			segmentIndex: number,
			pending?: boolean
		) => void;
		memoryInThinkingExpanded: (segmentKey: string) => boolean;
		toggleMemoryInThinking: (segmentKey: string) => void;
		thinkingActive: (pending?: boolean) => boolean;
		showThinkingSpinner: boolean;
		toolOutputExpanded: (item: Extract<ChatItem, { type: 'tool' }>) => boolean;
		toggleToolOutput: (id: string) => void;
		subagentExpanded: (id: string) => boolean;
		toggleSubagent: (id: string) => void;
		sessionId?: string;
		onNotifyAgent?: (payload: ChatTurnPayload) => void | Promise<void>;
		onStartJob?: (job: JobResource) => void | Promise<void>;
	} = $props();

	let firstEntry = $derived(timeline[0]);
	let childEntries = $derived(timeline.slice(1));

	function memoryLabel(memories: InjectedMemory[]) {
		return `Memories used · ${memories.length}`;
	}

	function parentLabel(entry: TimelineEntry) {
		if (entry.kind === 'reasoning') return 'Thinking';
		if (entry.kind === 'memory') return memoryLabel(entry.memories);
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
			{#if firstEntry.kind === 'reasoning' || firstEntry.kind === 'memory'}
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
						expanded={true}
						showSpinner={thinkingActive(firstEntry.pending) && showThinkingSpinner}
						contentOnly={true}
						onToggle={() =>
							toggleThinking(
								assistant,
								key,
								firstEntry.segmentIndex,
								firstEntry.pending
							)}
					/>
				{:else if firstEntry.kind === 'memory'}
					<MemoryCard
						memories={firstEntry.memories}
						expanded={true}
						contentOnly={true}
						nested={true}
						onToggle={() => {}}
					/>
				{:else if firstEntry.kind === 'tool'}
					<ToolFoldPanel
						item={firstEntry.tool}
						label={toolFoldLabel(firstEntry.tool)}
						expanded={toolOutputExpanded(firstEntry.tool)}
						nested={true}
						onToggle={() => toggleToolOutput(firstEntry.tool.id)}
						{sessionId}
						{onNotifyAgent}
						{onStartJob}
					/>
				{:else}
					<SubagentPanel
						item={firstEntry.subagent}
						expanded={subagentExpanded(firstEntry.subagent.id)}
						nested={true}
						onToggle={() => toggleSubagent(firstEntry.subagent.id)}
					/>
				{/if}
				{#each childEntries as entry (timelineEntryKey(entry))}
					{#if entry.kind === 'reasoning'}
						{@const key = segmentKey(entry)}
						<ThinkingBlock
							text={entry.text}
							pending={entry.pending}
							expanded={thinkingExpanded(
								assistant,
								key,
								entry.segmentIndex,
								entry.pending
							)}
							showSpinner={thinkingActive(entry.pending) && showThinkingSpinner}
							nested={true}
							onToggle={() =>
								toggleThinking(assistant, key, entry.segmentIndex, entry.pending)}
						/>
					{:else if entry.kind === 'memory'}
						{@const memoryKey = `${assistantId}-memory`}
						<MemoryCard
							memories={entry.memories}
							expanded={memoryInThinkingExpanded(memoryKey)}
							nested={true}
							onToggle={() => toggleMemoryInThinking(memoryKey)}
						/>
					{:else if entry.kind === 'tool'}
						<ToolFoldPanel
							item={entry.tool}
							label={toolFoldLabel(entry.tool)}
							expanded={toolOutputExpanded(entry.tool)}
							nested={true}
							onToggle={() => toggleToolOutput(entry.tool.id)}
							{sessionId}
							{onNotifyAgent}
							{onStartJob}
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
	/* Base .fold-panel / .fold-toggle / .fold-body styles live in
	   src/lib/styles/fold-panel.css. Only component-specific overrides here. */
	.activity-group-body {
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-self: stretch;
	}

	.activity-group-body :global(.thinking-panel.content-only .fold-body) {
		border: none;
		background: transparent;
		box-shadow: none;
		padding: 0;
	}
</style>
