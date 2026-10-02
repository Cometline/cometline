<script lang="ts">
	import { Check, Clipboard, Download, Trash2, TriangleAlert } from '@lucide/svelte';
	import type { MediaResource } from '#lib/client/cometmind.js';

	let {
		item,
		src,
		copied,
		onOpen,
		onCopy,
		onDownload,
		onDelete
	}: {
		item: MediaResource;
		src: string;
		copied: boolean;
		onOpen: () => void;
		onCopy: () => void;
		onDownload: () => void;
		onDelete: () => void;
	} = $props();
</script>

<article class="gallery-card">
	{#if item.kind === 'video'}
		<div class="gallery-video-wrap">
			<video
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
			{#if item.session_deleted}
				<span class="gallery-detached" title="Original session was deleted">
					<TriangleAlert size={14} />
				</span>
			{/if}
		</div>
	{:else}
		<button type="button" class="gallery-thumb" onclick={onOpen}>
			<img
				{src}
				alt={item.alt || 'Gallery image'}
				onerror={(event) => {
					const card = event.currentTarget.closest('button');
					if (card instanceof HTMLElement) card.dataset.missing = 'true';
				}}
			/>
			<span class="media-missing">This media was deleted.</span>
			{#if item.session_deleted}
				<span class="gallery-detached" title="Original session was deleted">
					<TriangleAlert size={14} />
				</span>
			{/if}
		</button>
	{/if}
	<div class="gallery-meta">
		<strong>{item.alt || (item.kind === 'video' ? 'Video' : 'Image')}</strong>
	</div>
	<div class="gallery-actions">
		<button
			type="button"
			class:copied
			title={item.kind === 'video' ? 'Copy video file' : 'Copy image'}
			onclick={onCopy}
		>
			{#if copied}
				<Check size={14} />
				Copied
			{:else}
				<Clipboard size={14} />
				Copy
			{/if}
		</button>
		<button type="button" onclick={onDownload}>
			<Download size={14} />
			Download
		</button>
		<button type="button" class="danger" onclick={onDelete}>
			<Trash2 size={14} />
			Delete
		</button>
	</div>
</article>

<style>
	.gallery-card {
		display: grid;
		gap: 10px;
		padding: 10px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: var(--panel-bg);
	}

	.gallery-thumb,
	.gallery-video-wrap {
		position: relative;
		width: 100%;
		aspect-ratio: 4 / 3;
		border: 0;
		padding: 0;
		border-radius: 10px;
		overflow: hidden;
		background: var(--color-111111);
		cursor: zoom-in;
	}

	.gallery-detached {
		position: absolute;
		top: 8px;
		right: 8px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--status-warning) 22%, var(--panel-bg));
		color: var(--status-warning);
	}

	.gallery-video-wrap {
		cursor: default;
	}

	.gallery-video-wrap video {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.gallery-thumb img {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.gallery-meta {
		display: grid;
		gap: 2px;
	}

	.gallery-meta strong {
		font-size: 13px;
	}

	.gallery-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.gallery-actions button {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: transparent;
		color: var(--text-main);
		font-size: 11px;
		padding: 5px 8px;
		cursor: pointer;
	}

	.gallery-actions button.copied {
		color: var(--status-success);
	}

	.gallery-actions .danger {
		color: var(--danger, var(--status-error));
	}

	.media-missing {
		display: none;
		margin: 0;
		padding: 18px 14px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--border-soft) 55%, transparent);
		color: var(--text-muted);
		font-size: 12px;
	}

	:global(.gallery-thumb[data-missing='true']) img,
	:global(.gallery-video-wrap[data-missing='true']) video {
		display: none;
	}

	:global(.gallery-thumb[data-missing='true']) .media-missing,
	:global(.gallery-video-wrap[data-missing='true']) .media-missing {
		display: block;
	}
</style>
