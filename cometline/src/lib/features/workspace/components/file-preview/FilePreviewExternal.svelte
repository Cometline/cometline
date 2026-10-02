<script lang="ts">
	import type { FilePreviewController } from '#lib/features/workspace/file-preview-controller.svelte.js';

	let { panel, mode }: { panel: FilePreviewController; mode: 'diff' | 'notice' } = $props();
</script>

{#if mode === 'diff' && panel.s.externalComparisonLines}
	<div class="external-diff-full-page" aria-label="External file comparison">
		<header class="external-diff-toolbar">
			<span>External change diff</span>
			<div class="external-change-actions">
				<button type="button" onclick={panel.reloadAfterExternalChange}>Reload</button>
				<button type="button" onclick={panel.keepEditingAfterExternalChange}
					>Keep editing</button
				>
				<button type="button" onclick={() => void panel.compareExternalChange()}>
					Close diff
				</button>
			</div>
		</header>
		<div class="external-diff-body">
			{#if panel.s.externalComparisonLines.length === 0}
				<p>No content differences found.</p>
			{:else}
				<!-- eslint-disable svelte/no-at-html-tags -- highlightGitDiffLines escapes every token -->
				<!-- prettier-ignore -->
				<pre class="external-diff" data-lang={panel.s.language ?? ''}><code>{#each panel.s.externalComparisonLines as line, i (i)}<span class="diff-line kind-{line.kind}">{#if line.prefix}<span class="diff-prefix">{line.prefix}</span>{/if}<span class="diff-code">{@html line.html}</span></span>{/each}</code></pre>
				<!-- eslint-enable svelte/no-at-html-tags -->
			{/if}
		</div>
	</div>
{:else if panel.s.externalChangePending}
	<div class="external-change-notice" role="status">
		<span>This file changed outside Cometline.</span>
		<div class="external-change-actions">
			<button type="button" onclick={panel.reloadAfterExternalChange}>Reload</button>
			<button type="button" onclick={panel.keepEditingAfterExternalChange}
				>Keep editing</button
			>
			<button type="button" onclick={() => void panel.compareExternalChange()}>Compare</button
			>
		</div>
	</div>
	{#if panel.s.externalComparisonError}
		<div class="file-preview-save-error">{panel.s.externalComparisonError}</div>
	{/if}
{/if}

<style>
	.file-preview-save-error {
		padding: 10px 14px;
		border-bottom: 1px solid rgba(180, 35, 24, 0.15);
		background: rgba(180, 35, 24, 0.05);
		color: var(--status-error);
		font-size: 12px;
	}

	.external-change-notice {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 8px 10px;
		border-bottom: 1px solid var(--border-soft);
		background: color-mix(in srgb, var(--status-warning) 10%, var(--panel-bg));
		color: var(--text-main);
		font-size: 12px;
	}

	.external-change-actions {
		display: flex;
		gap: 6px;
	}

	.external-change-actions button {
		border: 1px solid var(--border-soft);
		border-radius: 5px;
		padding: 3px 6px;
		background: var(--panel-bg);
		color: var(--text-main);
		font: inherit;
		cursor: pointer;
	}

	.external-change-actions button:hover {
		border-color: var(--text-soft);
	}

	.external-diff-full-page {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-height: 0;
	}

	.external-diff-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 6px 8px;
		border-bottom: 1px solid var(--border-soft);
		color: var(--text-main);
		font-size: 12px;
		font-weight: 600;
	}

	.external-diff-body {
		flex: 1;
		min-height: 0;
		overflow: auto;
		background: var(--panel-bg);
	}

	.external-diff-body p {
		margin: 0;
		padding: 10px;
		color: var(--text-muted);
		font-size: 12px;
	}

	.external-diff {
		margin: 0;
		overflow: auto;
		font: 11px/1.45 var(--font-mono, monospace);
		white-space: pre;
	}

	.diff-line {
		display: block;
		padding: 0 8px;
		white-space: pre;
	}

	.kind-meta,
	.kind-hunk,
	.kind-other,
	.kind-ctx {
		color: var(--text-muted);
	}

	.kind-add {
		background: color-mix(in srgb, var(--status-success) 16%, transparent);
	}

	.kind-del {
		background: color-mix(in srgb, var(--status-error) 14%, transparent);
	}

	.kind-add .diff-prefix {
		color: var(--status-success);
	}

	.kind-del .diff-prefix {
		color: var(--status-error);
	}

	@media (max-width: 640px) {
		.external-change-notice {
			align-items: flex-start;
			flex-direction: column;
		}
		.external-diff-toolbar {
			align-items: flex-start;
			flex-direction: column;
		}
	}
</style>
