<script lang="ts">
	import { Images } from '@lucide/svelte';
	import { onDestroy, onMount } from 'svelte';
	import { deleteMedia, listMedia, type MediaResource } from '$lib/client/cometmind';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import ImageLightbox from '$lib/features/chat/components/ImageLightbox.svelte';
	import GalleryCard from './GalleryCard.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import {
		copyImageToClipboard,
		copyMediaFileToClipboard,
		mediaContentURL
	} from '$lib/files/images';

	let items = $state<MediaResource[]>([]);
	let loading = $state(false);
	let error = $state('');
	let status = $state('');
	let copiedId = $state<string | null>(null);
	let copyResetTimer: ReturnType<typeof setTimeout> | null = null;
	let pendingDelete = $state<MediaResource | null>(null);
	let lightbox = $state<{ src: string; alt: string } | null>(null);

	function mediaSrc(item: MediaResource) {
		return mediaContentURL(item.id);
	}

	function deleteDescription(item: MediaResource | null) {
		if (item?.session_deleted) {
			return 'The file is removed from disk.';
		}
		return 'The file is removed from disk. The original chat keeps a deleted placeholder. Copies in other sessions stay.';
	}

	function downloadName(item: MediaResource) {
		const type = item.media_type || '';
		const ext =
			type.includes('jpeg') || type.includes('jpg')
				? 'jpg'
				: type.includes('gif')
					? 'gif'
					: type.includes('webp')
						? 'webp'
						: type.includes('webm')
							? 'webm'
							: item.kind === 'video'
								? 'mp4'
								: 'png';
		return `${item.alt || item.kind}-${item.id.slice(0, 8)}.${ext}`;
	}

	async function refresh() {
		loading = true;
		error = '';
		try {
			const result = await listMedia();
			items = [...(result.items ?? [])].sort((a, b) => b.created_at - a.created_at);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load gallery';
		} finally {
			loading = false;
		}
	}

	async function copyToClipboard(item: MediaResource) {
		status = '';
		try {
			if (item.kind === 'video') {
				await copyMediaFileToClipboard(item.storage_session_id, item.id);
			} else {
				await copyImageToClipboard(mediaSrc(item), item.media_type || 'image/png');
			}
			copiedId = item.id;
			if (copyResetTimer) clearTimeout(copyResetTimer);
			copyResetTimer = setTimeout(() => {
				copiedId = null;
				copyResetTimer = null;
			}, 1600);
		} catch (err) {
			copiedId = null;
			status = err instanceof Error ? err.message : 'Could not copy this file.';
		}
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		const target = pendingDelete;
		pendingDelete = null;
		status = '';
		try {
			await deleteMedia(target.id);
			await refresh();
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to delete media';
		}
	}

	function requestDelete(item: MediaResource) {
		if (settingsStore.settings.app.confirmBeforeDeletingMedia) {
			pendingDelete = item;
			return;
		}
		pendingDelete = item;
		void confirmDelete();
	}

	async function deleteWithoutFutureConfirmation() {
		void settingsStore.saveConfirmBeforeDeletingMedia(false).catch(() => {});
		await confirmDelete();
	}

	async function downloadItem(item: MediaResource) {
		let url = '';
		try {
			const response = await fetch(mediaSrc(item));
			if (!response.ok) {
				status = 'Could not download this file.';
				return;
			}
			const blob = await response.blob();
			url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = downloadName(item);
			link.click();
		} catch (err) {
			status = err instanceof Error ? err.message : 'Could not download this file.';
		} finally {
			if (url) URL.revokeObjectURL(url);
		}
	}

	onMount(() => {
		void refresh();
	});

	onDestroy(() => {
		if (copyResetTimer) clearTimeout(copyResetTimer);
	});
</script>

<div class="gallery-page settings-ui">
	<header class="gallery-header">
		<p>Every generated, presented, and captured still or clip, newest first.</p>
		{#if status}
			<p class="gallery-status">{status}</p>
		{/if}
		{#if error}
			<p class="gallery-error">{error}</p>
		{/if}
	</header>
	{#if loading}
		<section class="gallery-empty settings-panel-frame" aria-busy="true">
			<p>Loading…</p>
		</section>
	{:else if items.length === 0}
		<section class="gallery-empty settings-panel-frame">
			<Images size={28} stroke-width={1.6} />
			<h2>Nothing here yet</h2>
			<p>Generate or present media in a chat and it will show up here.</p>
		</section>
	{:else}
		<div class="gallery-grid">
			{#each items as item (item.id)}
				<GalleryCard
					{item}
					src={mediaSrc(item)}
					copied={copiedId === item.id}
					onOpen={() =>
						(lightbox = { src: mediaSrc(item), alt: item.alt || 'Gallery image' })}
					onCopy={() => void copyToClipboard(item)}
					onDownload={() => void downloadItem(item)}
					onDelete={() => requestDelete(item)}
				/>
			{/each}
		</div>
	{/if}
</div>

<ConfirmActionModal
	open={Boolean(pendingDelete)}
	title="Delete this media?"
	description={deleteDescription(pendingDelete)}
	confirmLabel="Delete"
	secondaryLabel="Don't ask again"
	onConfirm={() => void confirmDelete()}
	onSecondary={() => void deleteWithoutFutureConfirmation()}
	onCancel={() => (pendingDelete = null)}
/>

{#if lightbox}
	<ImageLightbox open src={lightbox.src} alt={lightbox.alt} onClose={() => (lightbox = null)} />
{/if}

<style>
	.gallery-page {
		display: flex;
		flex-direction: column;
		box-sizing: border-box;
		height: 100%;
		min-height: 0;
		min-width: 0;
		width: 100%;
		max-width: 100%;
		padding: 20px 24px;
		gap: 16px;
		overflow: hidden;
	}

	.gallery-header,
	.gallery-grid,
	.gallery-empty {
		width: 100%;
		min-width: 0;
	}

	.gallery-header {
		display: grid;
		gap: 6px;
		flex-shrink: 0;
	}

	.gallery-header > p {
		margin: 0;
		padding-left: 14px;
		color: var(--text-muted);
		font-size: 12px;
		line-height: 1.5;
	}

	.gallery-error {
		color: var(--danger, var(--status-error));
	}

	.gallery-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		min-height: 0;
		text-align: center;
		color: var(--text-muted);
	}

	.gallery-empty h2 {
		margin: 0;
		font-size: 15px;
		font-weight: 650;
		color: var(--text-main);
	}

	.gallery-empty p {
		margin: 0;
		max-width: 320px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.gallery-grid {
		flex: 1;
		min-height: 0;
		overflow: auto;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
		gap: 16px;
		align-content: start;
	}

	@container main-pane (max-width: 760px) {
		.gallery-page {
			padding: 16px;
		}
	}
</style>
