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
	import type { AssistantStackContext } from '$lib/conversation/assistant-stack-props';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import { resolveImageSrc } from '$lib/files/images';
	import ImageLightbox from '$lib/components/chat/ImageLightbox.svelte';
	import SelectionAddToChat from '$lib/components/SelectionAddToChat.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import {
		assistantResponseSource,
		buildAssistantResponseContext
	} from '$lib/conversation/assistant-response-context';
	import {
		firstSelectionClientRect,
		selectionPopupPosition
	} from '$lib/workspace/selection-popup';

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

	let lightbox = $state<{ src: string; alt: string } | null>(null);
	let selectionPopup = $state<{ top: number; left: number; text: string } | null>(null);

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

	function clearSelectionPopup() {
		selectionPopup = null;
	}

	function updateSelectionPopup(root: HTMLElement) {
		if (item.id === context.streamingAssistantId) {
			clearSelectionPopup();
			return;
		}
		const selection = window.getSelection();
		if (!selection || selection.isCollapsed || selection.rangeCount === 0) {
			clearSelectionPopup();
			return;
		}
		if (!root.contains(selection.anchorNode) || !root.contains(selection.focusNode)) {
			clearSelectionPopup();
			return;
		}
		const text = selection.toString().trim();
		if (!text) {
			clearSelectionPopup();
			return;
		}
		selectionPopup = {
			...selectionPopupPosition(
				firstSelectionClientRect(selection.getRangeAt(0)),
				window.innerWidth
			),
			text
		};
	}

	function selectableResponse(node: HTMLElement) {
		const update = () => updateSelectionPopup(node);
		node.addEventListener('mouseup', update);
		node.addEventListener('keyup', update);
		return {
			destroy() {
				node.removeEventListener('mouseup', update);
				node.removeEventListener('keyup', update);
			}
		};
	}

	function addSelectionToChat() {
		if (!selectionPopup) return;
		const contextRef = buildAssistantResponseContext({
			sessionId: context.sessionId,
			itemId: item.id,
			items: context.threadItems,
			selectedText: selectionPopup.text
		});
		if (contextRef) {
			shellStore.addWebContextForActive(contextRef);
			shellStore.requestComposerFocus();
		}
		clearSelectionPopup();
		window.getSelection()?.removeAllRanges();
	}
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
		<div class="assistant-image-gallery" class:single-image={item.images.length === 1}>
			<div class="assistant-images scrollbar-none">
				{#each item.images as image, imageIndex (`${item.id}-image-${image.id ?? imageIndex}`)}
					{@const src = resolveImageSrc(image, context.sessionId)}
					{@const alt = image.alt ?? image.name ?? 'Presented image'}
					<button
						type="button"
						class="bubble assistant-bubble image-open"
						aria-label={`View ${alt}`}
						onclick={() => (lightbox = { src, alt })}
					>
						<img {src} {alt} />
					</button>
				{/each}
			</div>
		</div>
	{/if}
	{#if item.text.trim()}
		<div
			use:selectableResponse
			class="bubble assistant-bubble"
			data-session-find-text
			role="article"
			aria-label="Assistant response"
		>
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

{#if selectionPopup}
	<SelectionAddToChat
		position={{ top: selectionPopup.top, left: selectionPopup.left }}
		onAdd={addSelectionToChat}
		onDismiss={clearSelectionPopup}
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
				color-mix(in srgb, var(--hero-composer-glow-color, #72c0ff) 45%, transparent);
		}
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

	.assistant-image-gallery {
		width: min(680px, 100%);
		max-width: 100%;
		align-self: flex-start;
	}

	.assistant-images {
		display: flex;
		gap: 8px;
		width: 100%;
		overflow-x: auto;
		overflow-y: hidden;
		scroll-snap-type: x mandatory;
		scroll-behavior: smooth;
	}

	.assistant-image-gallery.single-image {
		width: fit-content;
	}

	.single-image .assistant-images {
		overflow: visible;
	}

	.image-open {
		display: block;
		flex: 0 0 min(280px, 78%);
		width: 100%;
		margin: 0;
		padding: 0;
		line-height: 0;
		cursor: zoom-in;
		overflow: hidden;
		aspect-ratio: 4 / 3;
		scroll-snap-align: start;
	}

	.single-image .image-open {
		flex-basis: auto;
		width: fit-content;
		max-width: 100%;
		aspect-ratio: auto;
	}

	.image-open img {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.single-image .image-open img {
		width: auto;
		height: auto;
		/* Prefer cqi (assistant-stack width) over % — % fights fit-content and
		 * Tailwind preflight's img{max-width:100%} expands the bubble after load. */
		max-width: min(420px, 100cqi);
		max-height: 360px;
		object-fit: contain;
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
