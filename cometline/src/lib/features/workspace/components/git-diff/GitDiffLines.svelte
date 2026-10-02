<script lang="ts">
	import type { HighlightedDiffLine } from '#lib/features/workspace/git-diff-highlight.js';

	let {
		lines,
		language
	}: {
		lines: HighlightedDiffLine[];
		language: string | null;
	} = $props();
</script>

<!--
  Keep zero whitespace between .diff-line nodes: this is a <pre>, so
  newlines in the template would render as blank rows between every line.
-->
<!-- eslint-disable svelte/no-at-html-tags -- highlightGitDiffLines escapes every token -->
<pre class="git-diff-body scrollbar-none" data-lang={language ?? ''}><code
		><!-- prettier-ignore -->{#each lines as line, i (i)}<span class="diff-line kind-{line.kind}">{#if line.prefix}<span class="diff-prefix">{line.prefix}</span>{/if}<span class="diff-code">{@html line.html}</span></span>{/each}</code
	></pre>

<!-- eslint-enable svelte/no-at-html-tags -->

<style>
	.git-diff-body {
		flex: 1;
		min-height: 0;
		margin: 0;
		padding: 8px 0 16px;
		overflow: auto;
		font-size: 11.5px;
		line-height: 1.45;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		white-space: pre;
		user-select: text;
	}

	.diff-line {
		display: block;
		padding: 0 12px;
		white-space: pre;
	}

	.diff-prefix {
		display: inline;
	}

	.diff-code {
		display: inline;
	}

	.kind-meta,
	.kind-hunk,
	.kind-other {
		color: var(--text-muted);
	}

	.kind-add {
		background: rgba(34, 197, 94, 0.14);
	}

	.kind-add .diff-prefix {
		color: var(--status-success);
	}

	.kind-del {
		background: rgba(239, 68, 68, 0.14);
	}

	.kind-del .diff-prefix {
		color: var(--color-b91c1c);
	}

	.kind-ctx {
		color: var(--text-muted);
	}

	.kind-ctx .diff-prefix {
		color: var(--text-muted);
	}

	:global([data-theme='dark']) .kind-add,
	:global(.dark) .kind-add {
		background: rgba(34, 197, 94, 0.18);
	}

	:global([data-theme='dark']) .kind-add .diff-prefix,
	:global(.dark) .kind-add .diff-prefix {
		color: var(--color-86efac);
	}

	:global([data-theme='dark']) .kind-del,
	:global(.dark) .kind-del {
		background: rgba(239, 68, 68, 0.18);
	}

	:global([data-theme='dark']) .kind-del .diff-prefix,
	:global(.dark) .kind-del .diff-prefix {
		color: var(--color-fca5a5);
	}
</style>
