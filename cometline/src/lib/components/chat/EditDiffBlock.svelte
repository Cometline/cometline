<script lang="ts">
	import type { DiffArtifact } from '$lib/tools/parse-edit-diff';

	let { diff }: { diff: DiffArtifact } = $props();
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
		color: inherit;
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
		color: #15803d;
		background: rgba(34, 197, 94, 0.14);
	}

	.kind-del {
		color: #b91c1c;
		background: rgba(239, 68, 68, 0.14);
	}

	.kind-ctx,
	.kind-other {
		color: var(--text-muted);
	}

	:global([data-theme='dark']) .kind-add,
	:global(.dark) .kind-add {
		color: #86efac;
		background: rgba(34, 197, 94, 0.18);
	}

	:global([data-theme='dark']) .kind-del,
	:global(.dark) .kind-del {
		color: #fca5a5;
		background: rgba(239, 68, 68, 0.18);
	}
</style>
