import {
	renderMarkdown,
	renderUserText,
	type WorkspaceMarkdownResources
} from '$lib/markdown/render';
import { openLink } from '$lib/open-link';
import { openWorkspaceFilePreview } from '$lib/features/workspace/open-file-preview';
import { getCachedWikiFiles, refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';

export function createAssistantMarkdown(deps: {
	getSource: () => string;
	getStreaming: () => boolean;
	getMode: () => 'assistant' | 'user';
	getWikiFiles: () => readonly string[];
	getWorkspaceResources: () => WorkspaceMarkdownResources | null;
	getAnnotateSourceLines: () => boolean;
	getDeferred: () => boolean;
}) {
	const s = $state({
		cachedWikiFiles: getCachedWikiFiles() as string[],
		html: '',
		rendered: false,
		renderedSource: null as string | null,
		displaySource: '',
		snappedExistingSource: false
	});
	const effectiveWikiFiles = $derived(
		deps.getWikiFiles().length > 0 ? deps.getWikiFiles() : s.cachedWikiFiles
	);

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
	const userHtml = $derived(deps.getMode() === 'user' ? renderUserText(deps.getSource()) : '');

	// Intentionally start empty; $effect.pre snaps to `source` before first paint
	// so we never read the prop into $state() (avoids state_referenced_locally).
	let renderVersion = 0;
	let throttleTimer: ReturnType<typeof setTimeout> | null = null;
	let revealFrame = 0;
	let lastRenderAt = 0;

	async function render(text: string) {
		const files = effectiveWikiFiles;
		const resources = deps.getWorkspaceResources();
		const resourceKey = resources
			? `${resources.kind}\u0000${resources.workspacePath}\u0000${resources.filePath}`
			: '';
		const includeSourceLines =
			deps.getAnnotateSourceLines() && !deps.getStreaming() && resources !== null;
		const cacheKey = `${text}\u0000${files.join('\n')}\u0000${resourceKey}\u0000${includeSourceLines}`;
		if (s.rendered && s.renderedSource === cacheKey) return;
		const version = ++renderVersion;
		try {
			const next = await renderMarkdown(text, {
				wikiFiles: files,
				workspaceResources: resources ?? undefined,
				annotateSourceLines: includeSourceLines
			});
			if (version !== renderVersion) return;
			s.html = next;
			s.rendered = true;
			s.renderedSource = cacheKey;
		} catch {
			if (version !== renderVersion) return;
			// Leave the plaintext fallback visible on failure.
			s.rendered = false;
			s.renderedSource = null;
		}
	}

	function cancelScheduledRender() {
		if (throttleTimer) {
			clearTimeout(throttleTimer);
			throttleTimer = null;
		}
	}

	function scheduleRender(text: string) {
		if (!deps.getStreaming()) {
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
			if (!deps.getStreaming() || reducedMotion) {
				s.displaySource = target;
				return;
			}
			const remaining = target.length - s.displaySource.length;
			if (remaining <= 0) return;
			const chars = Math.min(
				REVEAL_MAX_CHARS_PER_FRAME,
				Math.max(1, Math.ceil(remaining / REVEAL_CATCHUP_FRAMES))
			);
			s.displaySource = target.slice(0, s.displaySource.length + chars);
			if (s.displaySource.length < target.length) {
				revealFrame = requestAnimationFrame(step);
			}
		};
		revealFrame = requestAnimationFrame(step);
	}

	// When remounting mid-stream (e.g. switching back to a session), snap to the
	// accumulated deps.getSource() once so we do not replay the typewriter from empty.
	$effect.pre(() => {
		if (deps.getMode() === 'user') return;
		if (s.snappedExistingSource || deps.getSource().length === 0) return;
		s.displaySource = deps.getSource();
		s.snappedExistingSource = true;
	});

	$effect(() => {
		// User deps.getMode() renders synchronously via the derived above; nothing to schedule.
		if (deps.getMode() === 'user') return;
		const target = deps.getSource();
		if (!deps.getStreaming() || reducedMotion) {
			cancelReveal();
			s.displaySource = target;
			return;
		}
		if (target.length < s.displaySource.length) {
			s.displaySource = target;
		}
		if (target.length > s.displaySource.length) {
			revealNextFrame(target);
		}
		return cancelReveal;
	});

	$effect(() => {
		if (deps.getMode() !== 'assistant' || !deps.getSource().includes('[[')) return;
		void refreshWikiFileIndex().then((files) => {
			s.cachedWikiFiles = files;
		});
	});

	$effect(() => {
		// User deps.getMode() renders synchronously via the derived above; nothing to schedule.
		if (deps.getMode() === 'user') return;
		// Hydration-only mega skip: keep plaintext under opacity:0; do not kick Shiki.
		// When `deps.getDeferred()` clears on this same instance, fall through to full render.
		if (deps.getDeferred()) {
			cancelScheduledRender();
			renderVersion += 1;
			s.html = '';
			s.rendered = false;
			s.renderedSource = null;
			return () => {
				cancelScheduledRender();
			};
		}
		const text = s.displaySource;
		// Re-evaluate when deps.getStreaming() flips so the final non-throttled render lands.
		void deps.getStreaming();
		void effectiveWikiFiles;
		void deps.getWorkspaceResources();
		void deps.getAnnotateSourceLines();
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

	return {
		s,
		get userHtml() {
			return userHtml;
		},
		onClick,
		onKeydown
	};
}
