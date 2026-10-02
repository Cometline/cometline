<script lang="ts">
	import { Check, Copy } from '@lucide/svelte';
	import AssistantMarkdown from '#lib/components/AssistantMarkdown.svelte';
	import AssistantThinkingWait from '#lib/features/chat/components/AssistantThinkingWait.svelte';
	import ToolFoldPanel from '#lib/features/chat/components/ToolFoldPanel.svelte';
	import AssistantActivityGroup from '#lib/features/chat/components/AssistantActivityGroup.svelte';
	import TimelineEntryRow from '#lib/features/chat/components/TimelineEntryRow.svelte';
	import { setReactiveChatTurnContext } from '#lib/features/chat/chat-turn-context.js';
	import { assistantThinkingWait } from '#lib/features/chat/thread-format.js';
	import {
		buildAssistantTimeline,
		pinnedJobProposalsForAssistant,
		shouldGroupAssistantTimeline
	} from '#lib/features/chat/thinking-attribution.js';
	import { timelineEntryKey } from '#lib/features/chat/thread-view-helpers.js';
	import type { AssistantStackContext } from '#lib/features/chat/assistant-stack-props.js';
	import type { ChatItem } from '#lib/stores/chat.svelte.js';
	import ImageLightbox from '#lib/features/chat/components/ImageLightbox.svelte';
	import SelectionAddToChat from '#lib/components/SelectionAddToChat.svelte';
	import { assistantResponseSource } from '#lib/features/chat/assistant-response-context.js';
	import { createAssistantStackSelection } from '#lib/features/chat/assistant-stack-selection.svelte.js';
	import AssistantImageGallery from '#lib/features/chat/components/assistant-stack/AssistantImageGallery.svelte';

	type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

	let {
		item,
		context,
		showActivitySpinner,
		deferMarkdown = false
	}: {
		item: AssistantItem;
		context: AssistantStackContext;
		showActivitySpinner: boolean;
		/** Skip AssistantMarkdown/Shiki while transcript is hydrating (mega only). */
		deferMarkdown?: boolean;
	} = $props();

	let lightbox = $state<{ src: string; alt: string } | null>(null);

	const selection = createAssistantStackSelection({
		getItemId: () => item.id,
		getStreamingAssistantId: () => context.streamingAssistantId,
		getSessionId: () => context.sessionId,
		getThreadItems: () => context.threadItems
	});

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
		!item.text.trim() &&
			!item.images?.length &&
			!(item.id === context.streamingAssistantId && context.sessionStreaming)
	);
	const responseSource = $derived(
		assistantResponseSource(context.sessionId, item.id, context.threadItems)
	);
</script>

<div class="assistant-stack" data-assistant-response-source={responseSource ?? undefined}>
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
	{#if item.images?.length}
		<AssistantImageGallery
			itemId={item.id}
			images={item.images}
			sessionId={context.sessionId}
			onOpenImage={(image) => (lightbox = image)}
		/>
	{/if}
	{#if item.text.trim()}
		<div
			use:selection.selectableResponse
			class="bubble assistant-bubble"
			data-session-find-text
			data-session-find-item={item.id}
			role="article"
			aria-label="Assistant response"
		>
			<AssistantMarkdown
				source={item.text}
				streaming={item.id === context.streamingAssistantId}
				deferred={deferMarkdown}
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
	{#if item.text.trim() && item.id !== context.streamingAssistantId}
		<div class="message-actions m-1">
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
			phase={item.activityPhase}
		/>
	{/if}
</div>

{#if selection.selectionPopup}
	<SelectionAddToChat
		position={{ top: selection.selectionPopup.top, left: selection.selectionPopup.left }}
		onAdd={selection.addSelectionToChat}
		onDismiss={selection.clearSelectionPopup}
	/>
{/if}

{#if lightbox}
	<ImageLightbox open src={lightbox.src} alt={lightbox.alt} onClose={() => (lightbox = null)} />
{/if}

<style>
	.assistant-stack {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 100%;
		min-width: 0;
		flex: 0 1 auto;
		align-items: flex-start;
		--assistant-activity-width: 80%;
		/* Definite inline size for image max-width: min(420px, 100cqi). */
		container-type: inline-size;
		container-name: assistant-stack;
	}

	.assistant-stack:global(.response-context-highlight) > .assistant-bubble {
		animation: response-context-highlight 1600ms var(--ease-smooth);
	}

	@keyframes response-context-highlight {
		0%,
		100% {
			box-shadow: none;
		}
		15%,
		70% {
			box-shadow: 0 0 0 3px
				color-mix(
					in srgb,
					var(--hero-composer-glow-color, var(--color-72c0ff)) 45%,
					transparent
				);
		}
	}

	.assistant-stack > :global(.memory-panel),
	.assistant-stack > :global(.tool-fold-panel),
	.assistant-stack > :global(.thinking-panel),
	.assistant-stack > :global(.subagent-panel) {
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
		align-self: flex-start;
		width: 60%;
		max-width: 100%;
		min-width: 0;
		box-sizing: border-box;
	}

	.assistant-stack :global(.activity-group > .fold-body) {
		align-self: flex-start;
		width: calc(100% - 20px);
		max-width: calc(100% - 20px);
		min-width: 0;
		box-sizing: border-box;
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

	.message-action :global(svg) {
		flex-shrink: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.message-actions {
			transition: none;
		}
	}
</style>
