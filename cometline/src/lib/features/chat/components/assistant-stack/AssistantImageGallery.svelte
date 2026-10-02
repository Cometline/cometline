<script lang="ts">
	import { onDestroy } from 'svelte';
	import { Check, Copy } from '@lucide/svelte';
	import type { ChatItem } from '#lib/stores/chat.svelte.js';
	import {
		copyImageToClipboard,
		copyMediaFileToClipboard,
		isVideoAttachment,
		resolveImageSrc
	} from '#lib/files/images.js';

	type AssistantImages = NonNullable<Extract<ChatItem, { type: 'assistant' }>['images']>;

	let {
		itemId,
		images,
		sessionId,
		onOpenImage
	}: {
		itemId: string;
		images: AssistantImages;
		sessionId: string;
		onOpenImage: (image: { src: string; alt: string }) => void;
	} = $props();

	let copiedImageKey = $state<string | null>(null);
	let copyResetTimer: ReturnType<typeof setTimeout> | null = null;

	async function copyMedia(key: string, copy: () => Promise<void>) {
		try {
			await copy();
			copiedImageKey = key;
			if (copyResetTimer) clearTimeout(copyResetTimer);
			copyResetTimer = setTimeout(() => {
				copiedImageKey = null;
				copyResetTimer = null;
			}, 1600);
		} catch {
			copiedImageKey = null;
		}
	}

	onDestroy(() => {
		if (copyResetTimer) clearTimeout(copyResetTimer);
	});
</script>

<div class="assistant-image-gallery" class:single-image={images.length === 1}>
	<div class="assistant-images scrollbar-none">
		{#each images as image, imageIndex (`${itemId}-image-${image.id ?? imageIndex}`)}
			{@const src = resolveImageSrc(image, sessionId)}
			{@const alt = image.alt ?? image.name ?? 'Presented image'}
			{@const mediaId = image.id}
			{#if isVideoAttachment(image)}
				<div class="assistant-video-wrap">
					<video
						class="assistant-video"
						{src}
						controls
						playsinline
						preload="metadata"
						onerror={(event) => {
							const host = event.currentTarget.parentElement;
							if (host) host.dataset.missing = 'true';
						}}
					>
						<track kind="captions" />
					</video>
					<p class="media-missing">This media was deleted.</p>
					{#if mediaId && sessionId}
						<button
							type="button"
							class="image-copy"
							class:copied={copiedImageKey === `${itemId}-${image.id ?? imageIndex}`}
							title="Copy video file"
							aria-label={`Copy ${alt}`}
							onclick={() =>
								void copyMedia(`${itemId}-${image.id ?? imageIndex}`, () =>
									copyMediaFileToClipboard(sessionId, mediaId)
								)}
						>
							{#if copiedImageKey === `${itemId}-${image.id ?? imageIndex}`}
								<Check size={13} />
								<span>Copied</span>
							{:else}
								<Copy size={13} />
								<span>Copy</span>
							{/if}
						</button>
					{/if}
				</div>
			{:else}
				<div class="image-card">
					<button
						type="button"
						class="bubble assistant-bubble image-open"
						aria-label={`View ${alt}`}
						onclick={() => onOpenImage({ src, alt })}
					>
						<img
							{src}
							{alt}
							onerror={(event) => {
								const host = event.currentTarget.closest('.assistant-images');
								const card = event.currentTarget.closest('.image-card');
								if (card instanceof HTMLElement) card.dataset.missing = 'true';
								if (host instanceof HTMLElement) host.dataset.hasMissing = 'true';
							}}
						/>
						<span class="media-missing">This media was deleted.</span>
					</button>
					<button
						type="button"
						class="image-copy"
						class:copied={copiedImageKey === `${itemId}-${image.id ?? imageIndex}`}
						title="Copy image"
						aria-label={`Copy ${alt}`}
						onclick={() =>
							void copyMedia(`${itemId}-${image.id ?? imageIndex}`, () =>
								copyImageToClipboard(src, image.media_type || 'image/png')
							)}
					>
						{#if copiedImageKey === `${itemId}-${image.id ?? imageIndex}`}
							<Check size={13} />
							<span>Copied</span>
						{:else}
							<Copy size={13} />
							<span>Copy</span>
						{/if}
					</button>
				</div>
			{/if}
		{/each}
	</div>
</div>

<style>
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

	.image-card {
		position: relative;
		flex: 0 0 min(280px, 78%);
		width: 100%;
		scroll-snap-align: start;
	}

	.assistant-video-wrap {
		position: relative;
		flex: 0 0 auto;
		scroll-snap-align: start;
	}

	.single-image .image-card {
		flex-basis: auto;
		width: fit-content;
		max-width: 100%;
	}

	.image-open {
		display: block;
		width: 100%;
		margin: 0;
		padding: 0;
		line-height: 0;
		cursor: zoom-in;
		overflow: hidden;
		aspect-ratio: 4 / 3;
	}

	.single-image .image-open {
		width: fit-content;
		max-width: 100%;
		aspect-ratio: auto;
	}

	.image-copy {
		position: absolute;
		top: 8px;
		right: 8px;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 8px;
		border: 1px solid transparent;
		border-radius: 7px;
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-soft);
		font-size: 11px;
		font-weight: 600;
		line-height: 1;
		cursor: pointer;
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.image-card:hover .image-copy,
	.assistant-video-wrap:hover .image-copy,
	.image-copy:focus-visible {
		opacity: 1;
	}

	.image-copy.copied {
		color: var(--status-success);
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

	.assistant-video {
		display: block;
		width: min(420px, 100%);
		max-height: 360px;
		border-radius: 12px;
		background: var(--color-000000);
	}

	.media-missing {
		display: none;
		margin: 0;
		padding: 18px 14px;
		border-radius: 12px;
		background: color-mix(in srgb, var(--border-soft) 55%, transparent);
		color: var(--text-muted);
		font-size: 12px;
	}

	:global(.assistant-images [data-missing='true']) img,
	:global(.assistant-images [data-missing='true']) video,
	:global(.image-card[data-missing='true']) img {
		display: none;
	}

	:global(.assistant-images [data-missing='true']) .media-missing,
	:global(.image-card[data-missing='true']) .media-missing {
		display: block;
	}

	:global(.image-card[data-missing='true']) .image-copy {
		display: none;
	}

	:global(.assistant-video-wrap[data-missing='true']) .image-copy {
		display: none;
	}
</style>
