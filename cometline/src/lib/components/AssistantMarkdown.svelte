<script lang="ts">
	import { renderMarkdown, renderUserText } from '$lib/markdown/render';
	import { openLink } from '$lib/open-link';
	import { openWorkspaceFilePreview } from '$lib/workspace/open-file-preview';

	let {
		source = '',
		streaming = false,
		mode = 'assistant'
	}: { source?: string; streaming?: boolean; mode?: 'assistant' | 'user' } = $props();

	// Throttle re-rendering while streaming so we don't reparse/highlight on every
	// token. A render version guards against stale async results overwriting newer
	// output when the highlighter resolves out of order.
	const STREAM_THROTTLE_MS = 40;
	const REVEAL_CATCHUP_FRAMES = 24;
	const REVEAL_MAX_CHARS_PER_FRAME = 4;

	const reducedMotion =
		typeof window !== 'undefined' &&
		window.matchMedia('(prefers-reduced-motion: reduce)').matches;

	// User messages render synchronously (no Shiki/async), so we compute their
	// HTML eagerly and show the embed chips on the very first paint — no flash of
	// raw text. Assistant messages use the async markdown pipeline below.
	let userHtml = $derived(mode === 'user' ? renderUserText(source) : '');

	let html = $state('');
	let rendered = $state(false);
	let displaySource = $state('');
	let snappedExistingSource = $state(false);
	let renderVersion = 0;
	let throttleTimer: ReturnType<typeof setTimeout> | null = null;
	let revealFrame = 0;
	let lastRenderAt = 0;

	async function render(text: string) {
		const version = ++renderVersion;
		try {
			const next = await renderMarkdown(text);
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

	function cancelReveal() {
		if (revealFrame) {
			cancelAnimationFrame(revealFrame);
			revealFrame = 0;
		}
	}

	function revealNextFrame(target: string) {
		cancelReveal();
		const step = () => {
			revealFrame = 0;
			if (!streaming || reducedMotion) {
				displaySource = target;
				return;
			}
			const remaining = target.length - displaySource.length;
			if (remaining <= 0) return;
			const chars = Math.min(
				REVEAL_MAX_CHARS_PER_FRAME,
				Math.max(1, Math.ceil(remaining / REVEAL_CATCHUP_FRAMES))
			);
			displaySource = target.slice(0, displaySource.length + chars);
			if (displaySource.length < target.length) {
				revealFrame = requestAnimationFrame(step);
			}
		};
		revealFrame = requestAnimationFrame(step);
	}

	// When remounting mid-stream (e.g. switching back to a session), snap to the
	// accumulated source once so we do not replay the typewriter from empty.
	$effect.pre(() => {
		if (mode === 'user') return;
		if (snappedExistingSource || source.length === 0) return;
		displaySource = source;
		snappedExistingSource = true;
	});

	$effect(() => {
		// User mode renders synchronously via the derived above; nothing to schedule.
		if (mode === 'user') return;
		const target = source;
		if (!streaming || reducedMotion) {
			cancelReveal();
			displaySource = target;
			return;
		}
		if (target.length < displaySource.length) {
			displaySource = target;
		}
		if (target.length > displaySource.length) {
			revealNextFrame(target);
		}
		return cancelReveal;
	});

	$effect(() => {
		// User mode renders synchronously via the derived above; nothing to schedule.
		if (mode === 'user') return;
		const text = displaySource;
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

		const fileChip = target.closest('[data-file-path]');
		if (fileChip instanceof HTMLElement) {
			event.preventDefault();
			const path = fileChip.getAttribute('data-file-path');
			if (path) openWorkspaceFilePreview(path);
			return;
		}

		const anchor = target.closest('a[data-external-link]');
		if (!anchor) return;
		const href = anchor.getAttribute('data-external-link');
		if (!href) return;
		event.preventDefault();
		openLink(href);
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		const target = event.target;
		if (!(target instanceof Element)) return;
		const fileChip = target.closest('[data-file-path]');
		if (!(fileChip instanceof HTMLElement)) return;
		event.preventDefault();
		const path = fileChip.getAttribute('data-file-path');
		if (path) openWorkspaceFilePreview(path);
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="markdown" class:user-text={mode === 'user'} onclick={onClick} onkeydown={onKeydown}>
	{#if mode === 'user'}
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html userHtml}
	{:else if rendered}
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html html}
	{:else}
		<span class="markdown-plain">{displaySource}</span>
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

	/* Opaque chip backgrounds so the blue user bubble does not bleed through. */
	.markdown.user-text :global(.link-embed),
	.markdown.user-text :global(.file-embed),
	.markdown.user-text :global(.skill-embed) {
		background: #ffffff;
	}

	.markdown.user-text :global(.link-embed:hover),
	.markdown.user-text :global(.file-embed:hover) {
		background: #ffffff;
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
		display: inline-flex;
		align-items: center;
		max-width: 16rem;
		vertical-align: middle;
		padding: 0.05em 0.45em;
		border: 1px solid rgba(16, 185, 129, 0.22);
		border-radius: 6px;
		background: rgba(16, 185, 129, 0.07);
		text-decoration: none;
		line-height: 1.4;
		color: #1d5c42;
		overflow: hidden;
		cursor: pointer;
		font-weight: 650;
	}

	.markdown :global(.file-embed:hover) {
		background: rgba(16, 185, 129, 0.12);
		border-color: rgba(16, 185, 129, 0.34);
	}

	.markdown :global(.file-embed-label) {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
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
		color: #31517a;
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
