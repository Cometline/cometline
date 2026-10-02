<script lang="ts">
	import FileTypeIcon from '#lib/features/workspace/components/FileTypeIcon.svelte';

	let {
		results,
		activeIndex,
		listEl = $bindable(null),
		onHover,
		onSelect
	}: {
		results: string[];
		activeIndex: number;
		listEl?: HTMLUListElement | null;
		onHover: (index: number) => void;
		onSelect: (path: string) => void;
	} = $props();

	function fileName(path: string): string {
		return path.split(/[/\\]/).filter(Boolean).pop() || path;
	}

	function fileDir(path: string): string {
		const parts = path.split(/[/\\]/).filter(Boolean);
		if (parts.length <= 1) return '';
		return parts.slice(0, -1).join('/');
	}
</script>

<ul class="file-search-list" role="listbox" aria-label="File results" bind:this={listEl}>
	{#each results as path, index (path)}
		<li role="option" aria-selected={index === activeIndex}>
			<button
				type="button"
				class="file-search-row"
				class:active={index === activeIndex}
				data-result-index={index}
				onmouseenter={() => onHover(index)}
				onclick={() => onSelect(path)}
				title={path}
			>
				<span class="file-search-icon" aria-hidden="true">
					<FileTypeIcon {path} size={16} />
				</span>
				<span class="file-search-labels">
					<span class="file-search-name">{fileName(path)}</span>
					{#if fileDir(path)}
						<span class="file-search-dir">{fileDir(path)}</span>
					{/if}
				</span>
			</button>
		</li>
	{/each}
</ul>

<style>
	.file-search-list {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: min(48vh, 360px);
		overflow: auto;
	}

	.file-search-row {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		border: none;
		border-radius: 8px;
		padding: 8px 10px;
		background: transparent;
		color: var(--text-primary, var(--color-111111));
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}

	.file-search-row:hover,
	.file-search-row.active {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, var(--workspace-group-color)) 14%,
			transparent
		);
	}

	.file-search-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 16px;
		width: 16px;
		height: 16px;
	}

	.file-search-labels {
		display: flex;
		align-items: baseline;
		gap: 8px;
		min-width: 0;
		flex: 1 1 auto;
	}

	.file-search-name {
		flex: 0 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 550;
	}

	.file-search-dir {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
		font-size: 12px;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}
</style>
