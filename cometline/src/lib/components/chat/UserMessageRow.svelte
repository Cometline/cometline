<script lang="ts">
	import { fly } from 'svelte/transition';
	import { Check, Copy } from '@lucide/svelte';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import MessageContextChips from '$lib/components/chat/MessageContextChips.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import ImageLightbox from '$lib/components/chat/ImageLightbox.svelte';
	import { imageDataURL } from '$lib/files/images';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	const BUBBLE_IN = { x: 20, y: 140, duration: 320 };

	let {
		item,
		avatarSrc,
		avatarSrcset,
		continuationRow = false,
		copiedId,
		onCopyMessage,
		/** Follow-ups only — first-turn reveal must not overlap the particle flight. */
		flyOnReveal = true
	}: {
		item: Extract<ChatItem, { type: 'user' }>;
		avatarSrc: string;
		avatarSrcset?: string;
		continuationRow?: boolean;
		copiedId: string | null;
		onCopyMessage: (id: string, text: string) => void | Promise<void>;
		flyOnReveal?: boolean;
	} = $props();

	let flightHidden = $derived(item.reveal === false);
	/** Only bubbles that were staged hidden should fly in on reveal — not history rows. */
	let stagedOnce = $state(false);
	let lightbox = $state<{ src: string; alt: string } | null>(null);

	$effect(() => {
		if (item.reveal === false) stagedOnce = true;
	});

	let playFlyIn = $derived(stagedOnce && flyOnReveal);
</script>

{#snippet bubbleBody()}
	{#if item.images?.length}
		<div class="user-images" class:text-following={Boolean(item.text)}>
			{#each item.images as image, imageIndex (`${item.id}-image-${image.id ?? imageIndex}`)}
				{@const src = imageDataURL(image)}
				{@const alt = image.name ?? 'Attached image'}
				<button
					type="button"
					class="image-open"
					aria-label={`View ${alt}`}
					onclick={() => (lightbox = { src, alt })}
				>
					<img {src} {alt} />
				</button>
			{/each}
		</div>
	{/if}
	{#if item.text?.trim()}
		<AssistantMarkdown source={item.text.trim()} mode="user" />
	{/if}
{/snippet}

<ThreadRow variant="user" {continuationRow} data-user-item-id={item.id}>
	<ThreadAvatar variant="gutter" {avatarSrc} {avatarSrcset} />
	<div class="user-stack">
		{#if item.contexts?.length}
			<div
				class="user-contexts"
				class:flight-hidden={flightHidden}
				class:text-following={Boolean(item.text) || Boolean(item.images?.length)}
			>
				<MessageContextChips contexts={item.contexts} align="end" />
			</div>
		{/if}
		{#if flightHidden}
			<!-- Staging placeholder for flight measure / layout; enter motion waits for reveal. -->
			<div
				class="bubble user-bubble flight-hidden"
				data-flight-target="user"
				data-flight-user-id={item.id}
			>
				{@render bubbleBody()}
			</div>
		{:else if playFlyIn}
			<div class="bubble user-bubble" in:fly={BUBBLE_IN}>
				{@render bubbleBody()}
			</div>
		{:else}
			<div class="bubble user-bubble">
				{@render bubbleBody()}
			</div>
		{/if}
		{#if item.text?.trim()}
			<div class="message-actions user-message-actions">
				<button
					type="button"
					class="message-action m-1"
					class:copied={copiedId === item.id}
					title="Copy message"
					aria-label="Copy message"
					onclick={() => onCopyMessage(item.id, item.text.trim())}
				>
					{#if copiedId === item.id}
						<Check size={13} />
						<span>Copied</span>
					{:else}
						<Copy size={13} />
						<span>Copy</span>
					{/if}
				</button>
			</div>
		{/if}
	</div>
</ThreadRow>

{#if lightbox}
	<ImageLightbox open src={lightbox.src} alt={lightbox.alt} onClose={() => (lightbox = null)} />
{/if}

<style>
	.user-stack {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: 1 1 auto;
		min-width: 0;
		max-width: var(--chat-assistant-column);
	}

	.flight-hidden {
		opacity: 0;
		pointer-events: none;
	}

	.user-contexts {
		width: 100%;
	}

	.user-contexts.text-following {
		margin-bottom: 8px;
	}

	.image-open {
		display: block;
		width: 100%;
		padding: 0;
		border: none;
		background: transparent;
		cursor: zoom-in;
	}

	.message-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: -2px;
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.user-stack:hover .message-actions,
	.message-actions:focus-within {
		opacity: 1;
	}

	.user-message-actions {
		justify-content: flex-end;
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
