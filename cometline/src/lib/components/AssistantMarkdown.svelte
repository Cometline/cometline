<script lang="ts">
	import { createAssistantMarkdown } from '$lib/components/assistant-markdown.svelte';
	import type { WorkspaceMarkdownResources } from '$lib/markdown/render';

	let {
		source = '',
		streaming = false,
		mode = 'assistant',
		wikiFiles = [],
		workspaceResources = null,
		annotateSourceLines = false,
		deferred = false
	}: {
		source?: string;
		streaming?: boolean;
		mode?: 'assistant' | 'user';
		wikiFiles?: readonly string[];
		/** When set, relative images/links resolve against this workspace/wiki file. */
		workspaceResources?: WorkspaceMarkdownResources | null;
		/** File-preview-only source line metadata for rendered selections. */
		annotateSourceLines?: boolean;
		/**
		 * Skip async markdown/Shiki (hydration-only mega skip). When this flips
		 * false on the same instance, full markdown runs in place — no remount.
		 */
		deferred?: boolean;
	} = $props();

	const view = createAssistantMarkdown({
		getSource: () => source,
		getStreaming: () => streaming,
		getMode: () => mode,
		getWikiFiles: () => wikiFiles,
		getWorkspaceResources: () => workspaceResources,
		getAnnotateSourceLines: () => annotateSourceLines,
		getDeferred: () => deferred
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="markdown"
	class:user-text={mode === 'user'}
	onclick={view.onClick}
	onkeydown={view.onKeydown}
>
	{#if mode === 'user'}
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html view.userHtml}
	{:else if view.s.rendered}
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html view.s.html}
	{:else}
		<span class="markdown-plain">{view.s.displaySource}</span>
	{/if}
</div>

<style>
	.markdown {
		font-size: inherit;
		line-height: 1.55;
		white-space: normal;
		word-break: break-word;
		overflow-wrap: anywhere;
	}

	.markdown-plain {
		white-space: pre-wrap;
	}

	/* User messages are literal text (only URLs become chips); keep newlines. */
	.markdown.user-text {
		white-space: pre-wrap;
		overflow-wrap: break-word;
		word-break: normal;
	}

	/* Opaque chip backgrounds so the blue user bubble does not bleed through. */
	.markdown.user-text :global(.link-embed),
	.markdown.user-text :global(.file-embed),
	.markdown.user-text :global(.skill-embed) {
		background: var(--panel-bg);
	}

	.markdown.user-text :global(.link-embed:hover),
	.markdown.user-text :global(.file-embed:hover) {
		background: var(--panel-bg);
		border-color: var(--text-soft);
	}

	/* Inline URL embed chip: favicon + label, aligned with the text baseline. */
	.markdown :global(.link-embed) {
		display: inline-flex;
		align-items: center;
		gap: 0.3em;
		max-width: 16rem;
		vertical-align: middle;
		padding: 0.05em 0.45em;
		border: 1px solid var(--border-soft);
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.6);
		text-decoration: none;
		line-height: 1.4;
		color: var(--text-main);
		overflow: hidden;
		cursor: pointer;
	}

	.markdown :global(.link-embed:hover) {
		background: rgba(255, 255, 255, 0.95);
		border-color: var(--text-soft);
	}

	.markdown :global(.link-embed-icon) {
		flex-shrink: 0;
		width: 1em;
		height: 1em;
		vertical-align: -0.15em;
		object-fit: contain;
		border-radius: 3px;
	}

	.markdown :global(.link-embed-label) {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		font-size: 0.95em;
	}

	.markdown :global(.file-embed) {
		display: inline;
		vertical-align: baseline;
		padding: 0.05em 0.45em;
		border: 1px solid rgba(16, 185, 129, 0.22);
		border-radius: 6px;
		background: rgba(16, 185, 129, 0.07);
		text-decoration: none;
		line-height: 1.4;
		color: var(--color-1d5c42);
		cursor: pointer;
		font-weight: 650;
		box-decoration-break: clone;
		-webkit-box-decoration-break: clone;
	}

	.markdown :global(.file-embed:hover) {
		background: rgba(16, 185, 129, 0.12);
		border-color: rgba(16, 185, 129, 0.34);
	}

	.markdown :global(.file-embed-broken) {
		border-color: rgba(148, 163, 184, 0.45);
		background: rgba(148, 163, 184, 0.1);
		color: var(--text-muted);
		cursor: default;
		font-weight: 550;
	}

	.markdown :global(.file-embed-broken:hover) {
		background: rgba(148, 163, 184, 0.1);
		border-color: rgba(148, 163, 184, 0.45);
	}

	.markdown :global(.file-embed-label) {
		white-space: normal;
		overflow-wrap: anywhere;
		word-break: break-word;
		font-size: 0.95em;
	}

	.markdown :global(.skill-embed) {
		display: inline-flex;
		align-items: center;
		max-width: 16rem;
		vertical-align: middle;
		padding: 0.05em 0.45em;
		border: 1px solid rgba(37, 99, 235, 0.18);
		border-radius: 6px;
		background: rgba(37, 99, 235, 0.06);
		line-height: 1.4;
		color: var(--color-31517a);
		overflow: hidden;
		font-weight: 650;
	}

	.markdown :global(.skill-embed-label) {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		font-size: 0.95em;
	}

	/* First/last child margin collapse so the bubble padding stays tight. */
	.markdown :global(> :first-child) {
		margin-top: 0;
	}

	.markdown :global(> :last-child) {
		margin-bottom: 0;
	}

	.markdown :global(p) {
		margin: 0 0 0.6em;
	}

	.markdown :global(ul),
	.markdown :global(ol) {
		margin: 0 0 0.6em;
		padding-left: 1.4em;
	}

	.markdown :global(li) {
		margin: 0.15em 0;
	}

	.markdown :global(li > p) {
		margin: 0;
	}

	.markdown :global(h1),
	.markdown :global(h2),
	.markdown :global(h3),
	.markdown :global(h4),
	.markdown :global(h5),
	.markdown :global(h6) {
		margin: 0.8em 0 0.4em;
		line-height: 1.3;
		font-weight: 650;
	}

	.markdown :global(h1) {
		font-size: 1.4em;
	}
	.markdown :global(h2) {
		font-size: 1.25em;
	}
	.markdown :global(h3) {
		font-size: 1.1em;
	}

	.markdown :global(a),
	.markdown :global(.md-workspace-link) {
		color: var(--accent);
		text-decoration: underline;
		text-underline-offset: 2px;
		cursor: pointer;
	}
</style>
