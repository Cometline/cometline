<script lang="ts">
	import type { ParsedEditDiff } from '$lib/tools/parse-edit-diff';

	let { diff }: { diff: ParsedEditDiff } = $props();
</script>

<div class="edit-diff">
	{#if diff.summary}
		<p class="edit-diff-summary">{diff.summary}</p>
	{/if}
	<pre class="edit-diff-body scrollbar-none"><code
			>{#each diff.lines as line, i (i)}<span class="diff-line kind-{line.kind}">{line.text}
</span>{/each}</code
		></pre>
</div>

<style>
	.edit-diff {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.edit-diff-summary {
		margin: 0;
		font-size: 12px;
		font-weight: 500;
		color: var(--text-primary, var(--text));
	}

	.edit-diff-body {
		margin: 0;
		font-size: 11.5px;
		line-height: 1.45;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		white-space: pre;
		overflow: auto;
		max-height: 280px;
		border-radius: 8px;
		border: 1px solid var(--border-soft);
		background: rgba(15, 23, 42, 0.04);
		padding: 6px 0;
	}

	.diff-line {
		display: block;
		padding: 0 10px;
		white-space: pre;
	}

	.kind-meta,
	.kind-hunk {
		color: var(--text-muted);
	}

	.kind-add {
		color: var(--status-success, #15803d);
		background: color-mix(in srgb, var(--status-success, #22c55e) 12%, transparent);
	}

	.kind-del {
		color: var(--status-error, #b91c1c);
		background: color-mix(in srgb, var(--status-error, #ef4444) 12%, transparent);
	}

	.kind-ctx,
	.kind-other {
		color: var(--text-muted);
	}
</style>
