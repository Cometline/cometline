<script lang="ts">
	import { materialIconUrlForPath } from '$lib/workspace/material-file-icon';

	let {
		path,
		size = 16,
		alt = ''
	}: {
		/** Workspace-relative or absolute file path (name + extension used). */
		path: string;
		size?: number;
		alt?: string;
	} = $props();

	const src = $derived(materialIconUrlForPath(path));
</script>

{#if src}
	<img
		class="file-type-icon"
		src={src}
		{alt}
		width={size}
		height={size}
		draggable="false"
	/>
{:else}
	<span class="file-type-icon-fallback" style:width="{size}px" style:height="{size}px" aria-hidden="true"
	></span>
{/if}

<style>
	.file-type-icon {
		display: block;
		flex: 0 0 auto;
		width: auto;
		height: auto;
		object-fit: contain;
		/* Material SVGs are already colored; keep crisp at 16px. */
		image-rendering: auto;
		user-select: none;
		pointer-events: none;
	}

	.file-type-icon-fallback {
		display: block;
		flex: 0 0 auto;
		border-radius: 3px;
		background: color-mix(in srgb, var(--text-muted) 18%, transparent);
	}
</style>
