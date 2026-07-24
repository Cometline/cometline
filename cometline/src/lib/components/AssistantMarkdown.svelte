<script lang="ts">
	import {
		renderMarkdown,
		renderUserText,
		type WorkspaceMarkdownResources
	} from '$lib/markdown/render';
	import { openLink } from '$lib/open-link';
	import { openWorkspaceFilePreview } from '$lib/workspace/open-file-preview';
	import { getCachedWikiFiles, refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';

	let {
		source = '',
		streaming = false,
		mode = 'assistant',
		wikiFiles = [],
		workspaceResources = null
	}: {
		source?: string;
		streaming?: boolean;
		mode?: 'assistant' | 'user';
		wikiFiles?: readonly string[];
		/** When set, relative images/links resolve against this workspace/wiki file. */
		workspaceResources?: WorkspaceMarkdownResources | null;
	} = $props();

	let cachedWikiFiles = $state<string[]>(getCachedWikiFiles());
	const effectiveWikiFiles = $derived(wikiFiles.length > 0 ? wikiFiles : cachedWikiFiles);

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
	let renderedSource = $state<string | null>(null);
	// Intentionally start empty; $effect.pre snaps to `source` before first paint
	// so we never read the prop into $state() (avoids state_referenced_locally).
	let displaySource = $state('');
	let snappedExistingSource = $state(false);
	let renderVersion = 0;
	let throttleTimer: ReturnType<typeof setTimeout> | null = null;
	let revealFrame = 0;
	let lastRenderAt = 0;

	async function render(text: string) {
		const files = effectiveWikiFiles;
		const resources = workspaceResources;
		const resourceKey = resources
			? `${resources.kind}\u0000${resources.workspacePath}\u0000${resources.filePath}`
			: '';
		const cacheKey = `${text}\u0000${files.join('\n')}\u0000${resourceKey}`;
		if (rendered && renderedSource === cacheKey) return;
		const version = ++renderVersion;
		try {
			const next = await renderMarkdown(text, {
				wikiFiles: files,
				workspaceResources: resources ?? undefined
			});
			if (version !== renderVersion) return;
			html = next;
			rendered = true;
			renderedSource = cacheKey;
		} catch {
			if (version !== renderVersion) return;
			// Leave the plaintext fallback visible on failure.
			rendered = false;
			renderedSource = null;
		}
	}

	function cancelScheduledRender() {
		if (throttleTimer) {
			clearTimeout(throttleTimer);
			throttleTimer = null;
		}
	}

	function scheduleRender(text: string) {
		if (!streaming) {
			cancelScheduledRender();
			void render(text);
			return;
		}
		const now = Date.now();
		const elapsed = now - lastRenderAt;
		cancelScheduledRender();
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
		if (mode !== 'assistant' || !source.includes('[[')) return;
		void refreshWikiFileIndex().then((files) => {
			cachedWikiFiles = files;
		});
	});

	$effect(() => {
		// User mode renders synchronously via the derived above; nothing to schedule.
		if (mode === 'user') return;
		const text = displaySource;
		// Re-evaluate when streaming flips so the final non-throttled render lands.
		void streaming;
		void effectiveWikiFiles;
		void workspaceResources;
		scheduleRender(text);
		return () => {
			cancelScheduledRender();
		};
	});

	async function copyCodeBlock(button: HTMLElement) {
		const text = button.closest('.md-code-block')?.querySelector('pre')?.textContent ?? '';
		if (!text) return;
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			return;
		}
		button.classList.add('is-copied');
		button.setAttribute('aria-label', 'Copied');
		setTimeout(() => {
			button.classList.remove('is-copied');
			button.setAttribute('aria-label', 'Copy code');
		}, 1600);
	}

	function onClick(event: MouseEvent) {
		const target = event.target;
		if (!(target instanceof Element)) return;

		const copyBtn = target.closest('[data-code-copy]');
		if (copyBtn instanceof HTMLElement) {
			event.preventDefault();
			void copyCodeBlock(copyBtn);
			return;
		}

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
		overflow-wrap: break-word;
		word-break: normal;
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
		display: inline;
		vertical-align: baseline;
		padding: 0.05em 0.45em;
		border: 1px solid rgba(16, 185, 129, 0.22);
		border-radius: 6px;
		background: rgba(16, 185, 129, 0.07);
		text-decoration: none;
		line-height: 1.4;
		color: #1d5c42;
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

	.markdown :global(a),
	.markdown :global(.md-workspace-link) {
		color: var(--accent);
		text-decoration: underline;
		text-underline-offset: 2px;
		cursor: pointer;
	}

	.markdown :global(blockquote) {
		position: relative;
		margin: 0 0 0.6em;
		padding: 0.45em 0.9em 0.45em 2.1em;
		border: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
		border-radius: 10px;
		background: color-mix(in srgb, var(--border-soft) 22%, transparent);
		color: var(--text-muted);
	}

	.markdown :global(blockquote::before) {
		content: '“';
		position: absolute;
		left: 0.65em;
		top: 0.2em;
		font-family: Georgia, serif;
		font-size: 1.35em;
		line-height: 1;
		color: color-mix(in srgb, var(--text-soft) 70%, transparent);
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
		color: var(--status-error);
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.88em;
	}

	/* Fenced code: wrapper + copy button from render.ts.
	   Icons match Lucide Copy/Check used by message-action. */
	.markdown :global(.md-code-block) {
		position: relative;
		margin: 0 0 0.6em;
	}

	.markdown :global(.md-code-copy) {
		--md-copy-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Crect width='14' height='14' x='8' y='8' rx='2' ry='2'/%3E%3Cpath d='M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2'/%3E%3C/svg%3E");
		--md-check-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M20 6 9 17l-5-5'/%3E%3C/svg%3E");
		position: absolute;
		top: 0.4em;
		right: 0.4em;
		z-index: 1;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.75rem;
		height: 1.75rem;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 7px;
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-soft);
		cursor: pointer;
		transition:
			color var(--duration-fast, 120ms) var(--ease-smooth, ease),
			background var(--duration-fast, 120ms) var(--ease-smooth, ease),
			border-color var(--duration-fast, 120ms) var(--ease-smooth, ease);
	}

	.markdown :global(.md-code-copy::before) {
		content: '';
		display: block;
		width: 13px;
		height: 13px;
		background-color: currentColor;
		mask: var(--md-copy-icon) center / contain no-repeat;
		-webkit-mask: var(--md-copy-icon) center / contain no-repeat;
	}

	.markdown :global(.md-code-copy:hover) {
		color: var(--text-main);
		border-color: var(--border-soft);
		background: #ffffff;
	}

	.markdown :global(.md-code-copy:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}

	.markdown :global(.md-code-copy.is-copied) {
		color: var(--status-success);
	}

	.markdown :global(.md-code-copy.is-copied::before) {
		mask: var(--md-check-icon) center / contain no-repeat;
		-webkit-mask: var(--md-check-icon) center / contain no-repeat;
	}

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

	.markdown :global(.md-code-block > pre) {
		margin: 0;
		padding-right: 2.4em;
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
