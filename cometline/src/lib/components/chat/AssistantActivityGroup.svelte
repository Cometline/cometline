<script lang="ts">
	import { fade, slide } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
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
	import MemoryCard from '$lib/components/chat/MemoryCard.svelte';
	import TimelineEntryRow from '$lib/components/chat/TimelineEntryRow.svelte';
	import { getChatTurnContext } from '$lib/conversation/chat-turn-context';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import type { TimelineEntry, InjectedMemory } from '$lib/conversation/thinking-attribution';
	import { subagentProgressLabel } from '$lib/conversation/subagent-display';

	let {
		assistant,
		assistantId,
		timeline,
		parentExpanded,
		onToggleParent,
		timelineEntryKey,
		showThinkingSpinner,
		maxVisibleReasoning = 0,
		cycling = false
	}: {
		assistant: Extract<ChatItem, { type: 'assistant' }>;
		assistantId: string;
		timeline: TimelineEntry[];
		parentExpanded: boolean;
		onToggleParent: () => void;
		timelineEntryKey: (entry: TimelineEntry) => string;
		showThinkingSpinner: boolean;
		maxVisibleReasoning?: number;
		cycling?: boolean;
	} = $props();

	const ctx = $derived(getChatTurnContext());
	const bodyId = $derived(`activity-group-body-${assistantId}`);

	let firstEntry = $derived(timeline[0]);
	let childEntries = $derived(timeline.slice(1));

	let slidingWindow = $derived(maxVisibleReasoning > 0 && parentExpanded);
	let visibleChildren = $derived(
		slidingWindow ? childEntries.slice(-maxVisibleReasoning) : childEntries
	);
	let hiddenCount = $derived(
		slidingWindow ? Math.max(0, childEntries.length - maxVisibleReasoning) : 0
	);

	function memoryLabel(memories: InjectedMemory[]) {
		return `Memories used · ${memories.length}`;
	}

	function parentLabel(entry: TimelineEntry) {
		if (entry.kind === 'reasoning') return 'Thinking';
		if (entry.kind === 'memory') return memoryLabel(entry.memories);
		if (entry.kind === 'tool') return ctx.toolFoldLabel(entry.tool);
		return subagentProgressLabel(entry.subagent);
	}

	function thinkingActive(pending?: boolean) {
		return pending === true;
	}

	const CHILD_FADE = { duration: 500 };
	const CHILD_SLIDE_IN = { duration: 350, easing: cubicOut };
	const CHILD_SLIDE_OUT = { duration: 280, easing: cubicOut };

	function timelineChildTransition(
		node: Element,
		{ memory, animate }: { memory: boolean; animate: boolean },
		options: { direction: 'in' | 'out' | 'both' }
	) {
		if (!animate) {
			return { duration: 0 };
		}
		if (options.direction === 'out') {
			return slide(node, CHILD_SLIDE_OUT);
		}
		return memory ? fade(node, CHILD_FADE) : slide(node, CHILD_SLIDE_IN);
	}
</script>

{#if firstEntry}
	<div class="fold-panel activity-group">
		<button
			type="button"
			class="fold-toggle activity-group-toggle"
			aria-expanded={parentExpanded}
			aria-controls={bodyId}
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
			<div id={bodyId} class="fold-body activity-group-body scrollbar-none">
				{#if firstEntry.kind === 'memory'}
					<MemoryCard
						memories={firstEntry.memories}
						expanded={true}
						contentOnly={true}
						nested={true}
						onToggle={() => {}}
						{cycling}
					/>
				{:else}
					<TimelineEntryRow
						entry={firstEntry}
						{assistant}
						{assistantId}
						nested={true}
						{showThinkingSpinner}
						{cycling}
					/>
				{/if}
				{#each visibleChildren as entry (timelineEntryKey(entry))}
					<div
						class="timeline-child"
						transition:timelineChildTransition={{
							memory: entry.kind === 'memory',
							animate: slidingWindow
						}}
					>
						<TimelineEntryRow
							{entry}
							{assistant}
							{assistantId}
							nested={true}
							{showThinkingSpinner}
							{cycling}
						/>
					</div>
				{/each}
				{#if hiddenCount > 0}
					<div class="hidden-indicator" in:slide={CHILD_SLIDE_IN}>
						+{hiddenCount} more
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.activity-group-toggle {
		max-width: none;
	}

	.activity-group-toggle > span {
		overflow: visible;
		text-overflow: clip;
	}

	.activity-group-body {
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-self: stretch;
		align-items: stretch;
		min-width: 0;
		max-height: 400px;
		overflow: hidden auto;
	}

	.activity-group-body > :global(*) {
		flex: 0 0 auto;
		min-width: 0;
	}

	.activity-group-body :global(.thinking-panel.content-only .fold-body) {
		border: none;
		background: transparent;
		box-shadow: none;
		padding: 0;
	}

	.timeline-child {
		display: flex;
		flex-direction: column;
		flex: 0 0 auto;
		min-width: 0;
		overflow: clip;
	}

	.hidden-indicator {
		font-size: 11px;
		color: var(--text-muted);
		text-align: center;
		padding: 2px 0;
		opacity: 0.7;
	}
</style>
