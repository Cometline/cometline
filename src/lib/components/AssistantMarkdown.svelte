<script lang="ts">
	import { renderMarkdown, renderUserText } from '$lib/markdown/render';

	let {
		source = '',
		streaming = false,
		mode = 'assistant'
	}: { source?: string; streaming?: boolean; mode?: 'assistant' | 'user' } = $props();

	// Throttle re-rendering while streaming so we don't reparse/highlight on every
	// token. A render version guards against stale async results overwriting newer
	// output when the highlighter resolves out of order.
	const STREAM_THROTTLE_MS = 40;

	let html = $state('');
	let rendered = $state(false);
	let renderVersion = 0;
	let throttleTimer: ReturnType<typeof setTimeout> | null = null;
	let lastRenderAt = 0;

	async function render(text: string) {
		const version = ++renderVersion;
		try {
			const next = mode === 'user' ? renderUserText(text) : await renderMarkdown(text);
			if (version !== renderVersion) return;
			html = next;
			rendered = true;
		} catch {
			if (version !== renderVersion) return;
			// Leave the plaintext fallback visible on failure.
			rendered = false;
		}
	}

	function scheduleRender(text: string) {
		if (!streaming) {
			if (throttleTimer) {
				clearTimeout(throttleTimer);
				throttleTimer = null;
			}
			void render(text);
			return;
		}
		const now = Date.now();
		const elapsed = now - lastRenderAt;
		if (throttleTimer) clearTimeout(throttleTimer);
		const run = () => {
			throttleTimer = null;
			lastRenderAt = Date.now();
			void render(text);
		};
		if (elapsed >= STREAM_THROTTLE_MS) {
			run();
		} else {
			throttleTimer = setTimeout(run, STREAM_THROTTLE_MS - elapsed);
		}
	}

	$effect(() => {
		const text = source;
		// Re-evaluate when streaming flips so the final non-throttled render lands.
		void streaming;
		scheduleRender(text);
		return () => {
			if (throttleTimer) {
				clearTimeout(throttleTimer);
				throttleTimer = null;
			}
		};
	});

	function onClick(event: MouseEvent) {
		const target = event.target;
		if (!(target instanceof Element)) return;
		const anchor = target.closest('a[data-external-link]');
		if (!anchor) return;
		const href = anchor.getAttribute('data-external-link');
		if (!href) return;
		event.preventDefault();
		void window.electronAPI?.openExternal?.(href);
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="markdown" class:user-text={mode === 'user'} onclick={onClick}>
	{#if rendered}
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html html}
	{:else}
		<span class="markdown-plain">{source}</span>
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

	.markdown :global(a) {
		color: var(--accent);
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.markdown :global(blockquote) {
		margin: 0 0 0.6em;
		padding: 0.2em 0.9em;
		border-left: 3px solid var(--border-soft);
		color: var(--text-muted);
	}

	.markdown :global(hr) {
		border: none;
		border-top: 1px solid var(--border-soft);
		margin: 0.9em 0;
	}

	/* Inline code */
	.markdown :global(code) {
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.88em;
		background: rgba(15, 23, 42, 0.06);
		padding: 0.12em 0.36em;
		border-radius: 5px;
	}

	.markdown :global(kbd) {
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.8em;
		line-height: 1;
		padding: 0.2em 0.45em;
		border: 1px solid var(--border-soft);
		border-bottom-width: 2px;
		border-radius: 5px;
		background: #fafafa;
		color: var(--text-main);
		white-space: nowrap;
	}

	.markdown :global(mark) {
		background: #fff3a3;
		color: inherit;
		padding: 0.05em 0.2em;
		border-radius: 3px;
	}

	/* Block math: allow horizontal scroll for wide equations. */
	.markdown :global(.katex-display) {
		margin: 0.6em 0;
		overflow-x: auto;
		overflow-y: hidden;
		padding: 0.2em 0;
	}

	.markdown :global(.math-error) {
		color: #b42318;
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.88em;
	}

	/* Fenced code blocks: Shiki emits <pre class="shiki"><code>…</code></pre>. */
	.markdown :global(pre) {
		margin: 0 0 0.6em;
		padding: 0.7em 0.85em;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		overflow-x: auto;
		background: #ffffff;
		font-size: 0.86em;
		line-height: 1.5;
	}

	.markdown :global(pre.shiki) {
		background: #ffffff !important;
	}

	.markdown :global(pre code) {
		display: block;
		background: transparent;
		padding: 0;
		border-radius: 0;
		font-size: inherit;
		white-space: pre;
	}

	.markdown :global(table) {
		border-collapse: collapse;
		margin: 0 0 0.6em;
		font-size: 0.92em;
		display: block;
		max-width: 100%;
		overflow-x: auto;
	}

	.markdown :global(th),
	.markdown :global(td) {
		border: 1px solid var(--border-soft);
		padding: 0.35em 0.6em;
		text-align: left;
	}

	.markdown :global(th) {
		background: rgba(15, 23, 42, 0.03);
		font-weight: 650;
	}

	.markdown :global(img) {
		max-width: 100%;
		border-radius: 8px;
	}
</style>
