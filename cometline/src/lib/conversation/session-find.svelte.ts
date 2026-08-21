import { tick } from 'svelte';
import {
	findSessionTextMatches,
	SESSION_FIND_ACTIVE_HIGHLIGHT,
	SESSION_FIND_MATCH_HIGHLIGHT,
	type SessionFindMatch
} from './session-find';

type HighlightRegistry = {
	set(name: string, highlight: unknown): void;
	delete(name: string): void;
};

type HighlightConstructor = new (...ranges: Range[]) => unknown;

function highlightRegistry(): HighlightRegistry | null {
	if (typeof CSS === 'undefined') return null;
	return (CSS as unknown as { highlights?: HighlightRegistry }).highlights ?? null;
}

function highlightConstructor(): HighlightConstructor | null {
	return (globalThis as unknown as { Highlight?: HighlightConstructor }).Highlight ?? null;
}

export function createSessionFindController(getRoot: () => HTMLElement | null) {
	let open = $state(false);
	let query = $state('');
	let matches = $state.raw<SessionFindMatch[]>([]);
	let activeIndex = $state(-1);
	let focusRequestId = $state(0);
	let previousFocus: HTMLElement | null = null;

	function clearHighlights() {
		const registry = highlightRegistry();
		registry?.delete(SESSION_FIND_MATCH_HIGHLIGHT);
		registry?.delete(SESSION_FIND_ACTIVE_HIGHLIGHT);
	}

	function paintHighlights() {
		clearHighlights();
		const registry = highlightRegistry();
		const Highlight = highlightConstructor();
		if (!registry || !Highlight || matches.length === 0) return;
		registry.set(
			SESSION_FIND_MATCH_HIGHLIGHT,
			new Highlight(...matches.map((match) => match.range))
		);
		const active = matches[activeIndex];
		if (active) registry.set(SESSION_FIND_ACTIVE_HIGHLIGHT, new Highlight(active.range));
	}

	async function scrollActiveIntoView() {
		const active = matches[activeIndex];
		if (!active) return;
		const scroller = getRoot();
		if (!scroller) return;
		const expandButton = active.root.querySelector<HTMLButtonElement>(
			'[data-user-message-expand][aria-expanded="false"]'
		);
		if (expandButton) {
			expandButton.click();
			await tick();
			if (matches[activeIndex] !== active) return;
		}
		const rangeRect = active.range.getBoundingClientRect?.();
		const scrollerRect = scroller.getBoundingClientRect();
		if (rangeRect && (rangeRect.width > 0 || rangeRect.height > 0)) {
			scroller.scrollBy({
				top: rangeRect.top - scrollerRect.top - scrollerRect.height / 2,
				behavior: 'smooth'
			});
			return;
		}
		active.root.scrollIntoView({ block: 'center', behavior: 'smooth' });
	}

	function rebuild(options: { preserveIndex?: boolean; scroll?: boolean } = {}) {
		const root = getRoot();
		matches = root && query.trim() ? findSessionTextMatches(root, query) : [];
		if (matches.length === 0) activeIndex = -1;
		else if (!options.preserveIndex || activeIndex < 0) activeIndex = 0;
		else activeIndex = Math.min(activeIndex, matches.length - 1);
		paintHighlights();
		if (options.scroll && activeIndex >= 0) void scrollActiveIntoView();
	}

	function openFind() {
		if (!open) previousFocus = document.activeElement as HTMLElement | null;
		open = true;
		focusRequestId += 1;
		rebuild({ preserveIndex: true });
	}

	function closeFind(options: { restoreFocus?: boolean } = { restoreFocus: true }) {
		open = false;
		query = '';
		matches = [];
		activeIndex = -1;
		clearHighlights();
		if (options.restoreFocus !== false && previousFocus?.isConnected) previousFocus.focus();
		previousFocus = null;
	}

	function setQuery(next: string) {
		query = next;
		rebuild({ scroll: true });
	}

	function move(direction: 1 | -1) {
		if (matches.length === 0) return;
		activeIndex = (activeIndex + direction + matches.length) % matches.length;
		paintHighlights();
		void scrollActiveIntoView();
	}

	function observe() {
		const root = getRoot();
		if (!root || typeof MutationObserver === 'undefined') return () => {};
		let timer: ReturnType<typeof setTimeout> | null = null;
		const observer = new MutationObserver(() => {
			if (timer) return;
			timer = setTimeout(() => {
				timer = null;
				rebuild({ preserveIndex: true });
			}, 120);
		});
		observer.observe(root, { childList: true, characterData: true, subtree: true });
		return () => {
			observer.disconnect();
			if (timer) clearTimeout(timer);
		};
	}

	return {
		get open() {
			return open;
		},
		get query() {
			return query;
		},
		get matchCount() {
			return matches.length;
		},
		get activeIndex() {
			return activeIndex;
		},
		get focusRequestId() {
			return focusRequestId;
		},
		openFind,
		closeFind,
		setQuery,
		next: () => move(1),
		previous: () => move(-1),
		observe,
		rebuild
	};
}

export type SessionFindController = ReturnType<typeof createSessionFindController>;
