<script lang="ts">
	import { Check, Copy } from '@lucide/svelte';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import AssistantThinkingWait from '$lib/components/chat/AssistantThinkingWait.svelte';
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import AssistantActivityGroup from '$lib/components/chat/AssistantActivityGroup.svelte';
	import TimelineEntryRow from '$lib/components/chat/TimelineEntryRow.svelte';
	import { setReactiveChatTurnContext } from '$lib/conversation/chat-turn-context';
	import { assistantThinkingWait } from '$lib/conversation/thread-format';
	import {
		buildAssistantTimeline,
		pinnedJobProposalsForAssistant,
		shouldGroupAssistantTimeline
	} from '$lib/conversation/thinking-attribution';
	import { timelineEntryKey } from '$lib/conversation/thread-view-helpers';
	import { memoryUpdateHint, memoryUpdateTooltip } from '$lib/memory-updates';
	import type { AssistantStackContext } from '$lib/conversation/assistant-stack-props';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

	let {
		item,
		context,
		showActivitySpinner
	}: {
		item: AssistantItem;
		context: AssistantStackContext;
		showActivitySpinner: boolean;
	} = $props();

	setReactiveChatTurnContext(() => context);

	const timeline = $derived(
		buildAssistantTimeline(item.id, context.threadItems, context.thinkingForAssistant)
	);
	const grouped = $derived(shouldGroupAssistantTimeline(item, timeline));
	const pinnedJobTools = $derived(pinnedJobProposalsForAssistant(item.id, context.threadItems));
	const maxVisible = $derived(
		item.id === context.streamingAssistantId && context.sessionStreaming ? 3 : 0
	);
	const cycling = $derived(item.id === context.streamingAssistantId && context.sessionStreaming);
	const thinkingWait = $derived(assistantThinkingWait(item, context.now));
	const showThinkingSpinner = $derived(
		!item.text.trim() && !(item.id === context.streamingAssistantId && context.sessionStreaming)
	);
</script>

<div class="assistant-stack">
	{#if grouped}
		<AssistantActivityGroup
			assistant={item}
			assistantId={item.id}
			{timeline}
			parentExpanded={context.fold.activityGroupExpanded(item.id, item)}
			onToggleParent={() => context.fold.toggleActivityGroup(item.id, item)}
			{timelineEntryKey}
			{showThinkingSpinner}
			maxVisibleReasoning={maxVisible}
			{cycling}
		/>
	{:else}
		{#each timeline as entry (timelineEntryKey(entry))}
			<TimelineEntryRow
				{entry}
				assistant={item}
				assistantId={item.id}
				{showThinkingSpinner}
			/>
		{/each}
	{/if}
	{#if item.text}
		<div class="bubble assistant-bubble">
			<AssistantMarkdown
				source={item.text}
				streaming={item.id === context.streamingAssistantId}
			/>
		</div>
	{/if}
	{#each pinnedJobTools as jobTool (jobTool.id)}
		<ToolFoldPanel
			item={jobTool}
			label={context.toolFoldLabel(jobTool)}
			expanded={context.fold.toolOutputExpanded(jobTool)}
			onToggle={() => context.fold.toggleToolOutput(jobTool.id)}
			sessionId={context.sessionId}
			onNotifyAgent={context.onNotifyAgent}
			onStartJob={context.onStartJob}
		/>
	{/each}
	{#if item.text && item.id !== context.streamingAssistantId}
		<div class="message-actions m-1">
			{#if item.memoryUpdates?.length}
				<span
					class="message-action memory-hint"
					title={memoryUpdateTooltip(item.memoryUpdates)}
					aria-label={memoryUpdateTooltip(item.memoryUpdates)}
				>
					{memoryUpdateHint(item.memoryUpdates)}
				</span>
			{/if}
			<button
				type="button"
				class="message-action m-1"
				class:copied={context.copiedId === item.id}
				title="Copy message"
				aria-label="Copy message"
				onclick={() => context.onCopyMessage(item.id, item.text)}
			>
				{#if context.copiedId === item.id}
					<Check size={13} />
					<span>Copied</span>
				{:else}
					<Copy size={13} />
					<span>Copy</span>
				{/if}
			</button>
		</div>
	{/if}
	{#if showActivitySpinner}
		<AssistantThinkingWait
			label={thinkingWait.label}
			detail={thinkingWait.detail}
			color={context.heroGlowColor}
		/>
	{/if}
</div>

<style>
	.assistant-stack {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 100%;
		max-width: var(--chat-assistant-column);
		min-width: 0;
		flex: 0 1 auto;
		align-items: flex-start;
		--assistant-activity-width: 80%;
	}

	.assistant-stack > :global(.memory-panel),
	.assistant-stack > :global(.tool-fold-panel),
	.assistant-stack > :global(.thinking-panel),
	.assistant-stack > :global(.subagent-panel),
	.assistant-stack :global(.activity-group > .fold-body) {
		align-self: flex-start;
		width: var(--assistant-activity-width);
		max-width: 100%;
		min-width: 0;
		box-sizing: border-box;
	}

	.assistant-stack > :global(.memory-panel .memory-body) {
		width: 100%;
		box-sizing: border-box;
	}

	.assistant-stack :global(.activity-group) {
		align-self: stretch;
		width: 100%;
		min-width: 0;
	}

	.message-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: -2px;
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.assistant-stack:hover .message-actions,
	.message-actions:focus-within {
		opacity: 1;
	}

	.message-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border: 1px solid transparent;
		border-radius: 7px;
		background: transparent;
		color: var(--text-soft);
		font-size: 11px;
		font-weight: 600;
		line-height: 1;
		cursor: pointer;
		transition:
			color var(--duration-fast) var(--ease-smooth),
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.message-action:hover {
		color: var(--text-main);
		background: rgba(255, 255, 255, 0.92);
		border-color: var(--border-soft);
	}

	.message-action.copied {
		color: var(--status-success);
	}

	.memory-hint {
		cursor: default;
	}

	.memory-hint:hover {
		background: transparent;
		border-color: transparent;
		color: var(--text-soft);
	}

	.message-action :global(svg) {
		flex-shrink: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.message-actions {
			transition: none;
		}
	}
</style>
