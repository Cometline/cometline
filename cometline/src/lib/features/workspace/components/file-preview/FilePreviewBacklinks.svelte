<script lang="ts">
	import { openWorkspaceFilePreview } from '$lib/features/workspace/open-file-preview';
	import { toWikiUiPath } from '$lib/wiki/paths';
	import { wikiStemFromPath } from '$lib/wiki/wikilinks';
	import type { FilePreviewController } from '$lib/features/workspace/file-preview-controller.svelte';

	let { panel, inSource = false }: { panel: FilePreviewController; inSource?: boolean } =
		$props();
</script>

<section class="backlinks" class:in-source={inSource} aria-label="Backlinks">
	<h3 class="backlinks-title">Backlinks</h3>
	{#if panel.s.backlinksLoading}
		<p class="backlinks-empty">Loading backlinks…</p>
	{:else if panel.s.backlinks.length === 0}
		<p class="backlinks-empty">No backlinks yet.</p>
	{:else}
		<ul class="backlinks-list">
			{#each panel.s.backlinks as linkPath (linkPath)}
				<li>
					<button
						type="button"
						class="backlink-item"
						onclick={() => openWorkspaceFilePreview(toWikiUiPath(linkPath))}
					>
						{wikiStemFromPath(linkPath)}
						<span class="backlink-path">{linkPath}</span>
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.backlinks {
		margin-top: 28px;
		padding-top: 16px;
		border-top: 1px solid rgba(0, 0, 0, 0.08);
	}

	.backlinks-title {
		margin: 0 0 10px;
		font-size: 12px;
		font-weight: 650;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--text-muted);
	}

	.backlinks-empty {
		margin: 0;
		font-size: 13px;
		color: var(--text-muted);
	}

	.backlinks-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.backlink-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		width: 100%;
		border: none;
		border-radius: 8px;
		padding: 8px 10px;
		background: rgba(0, 0, 0, 0.02);
		color: var(--text-primary, var(--color-111111));
		font-size: 13px;
		font-weight: 550;
		text-align: left;
		cursor: pointer;
	}

	.backlink-item:hover {
		background: rgba(0, 0, 0, 0.05);
	}

	.backlink-path {
		font-size: 11px;
		font-weight: 450;
		color: var(--text-muted);
	}
	.backlinks.in-source {
		flex: 0 0 auto;
		padding: 0 18px 24px;
	}
</style>
