import { tick } from 'svelte';
import {
	findSessionItemMatches,
	findSessionTextMatches,
	SESSION_FIND_ACTIVE_HIGHLIGHT,
	SESSION_FIND_MATCH_HIGHLIGHT,
	type SessionFindMatch,
	type SessionItemFindMatch
} from './session-find';
import type { ThreadTurn } from './thread-turns';

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

export interface SessionFindDeps {
	getRoot: () => HTMLElement | null;
	/** Transcript turns for store-backed search (virtualized threads). */
	getTurns: () => readonly ThreadTurn[];
}

export function createSessionFindController(deps: SessionFindDeps) {
	let open = $state(false);
	let query = $state('');
	let itemMatches = $state.raw<SessionItemFindMatch[]>([]);
	let activeIndex = $state(-1);
	let focusRequestId = $state(0);
	let previousFocus: HTMLElement | null = null;
	let paintVersion = 0;

	function clearHighlights() {
		const registry = highlightRegistry();
		registry?.delete(SESSION_FIND_MATCH_HIGHLIGHT);
		registry?.delete(SESSION_FIND_ACTIVE_HIGHLIGHT);
	}

	function activeItemMatch(): SessionItemFindMatch | null {
		return itemMatches[activeIndex] ?? null;
	}

	function occurrenceDomMatches(
		root: HTMLElement,
		itemId: string,
		needle: string
	): SessionFindMatch[] {
		return findSessionTextMatches(root, needle).filter(
			(match) => match.root.getAttribute('data-session-find-item') === itemId
		);
	}

	function paintHighlights() {
		clearHighlights();
		const registry = highlightRegistry();
		const Highlight = highlightConstructor();
		const root = deps.getRoot();
		const active = activeItemMatch();
		if (!registry || !Highlight || !root || !query.trim() || !active) return;

		const allMounted = findSessionTextMatches(root, query);
		if (allMounted.length > 0) {
			registry.set(
				SESSION_FIND_MATCH_HIGHLIGHT,
				new Highlight(...allMounted.map((match) => match.range))
			);
		}
		const forItem = occurrenceDomMatches(root, active.itemId, query);
		const activeDom = forItem[active.occurrenceInItem] ?? forItem[0];
		if (activeDom) {
			registry.set(SESSION_FIND_ACTIVE_HIGHLIGHT, new Highlight(activeDom.range));
		}
	}

	async function scrollActiveIntoView() {
		const active = activeItemMatch();
		if (!active) return;
		const version = ++paintVersion;
		// Allow ChatThread to force-mount + scroll the target turn first.
		await tick();
		await tick();
		if (version !== paintVersion) return;

		const scroller = deps.getRoot();
		if (!scroller) return;
		const itemAttr =
			typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
				? CSS.escape(active.itemId)
				: active.itemId.replace(/["\\]/g, '\\$&');
		const searchRoot = scroller.querySelector<HTMLElement>(
			`[data-session-find-item="${itemAttr}"]`
		);
		if (!searchRoot) {
			paintHighlights();
			return;
		}

		const expandButton = searchRoot.querySelector<HTMLButtonElement>(
			'[data-user-message-expand][aria-expanded="false"]'
		);
		if (expandButton) {
			expandButton.click();
			await tick();
			if (version !== paintVersion) return;
		}

		paintHighlights();
		const forItem = occurrenceDomMatches(scroller, active.itemId, query);
		const activeDom = forItem[active.occurrenceInItem] ?? forItem[0];
		if (activeDom) {
			const rangeRect = activeDom.range.getBoundingClientRect?.();
			const scrollerRect = scroller.getBoundingClientRect();
			if (rangeRect && (rangeRect.width > 0 || rangeRect.height > 0)) {
				scroller.scrollBy({
					top: rangeRect.top - scrollerRect.top - scrollerRect.height / 2,
					behavior: 'smooth'
				});
				return;
			}
			activeDom.root.scrollIntoView({ block: 'center', behavior: 'smooth' });
			return;
		}
		searchRoot.scrollIntoView({ block: 'center', behavior: 'smooth' });
	}

	function rebuild(options: { preserveIndex?: boolean; scroll?: boolean } = {}) {
		itemMatches = query.trim() ? findSessionItemMatches(deps.getTurns(), query) : [];
		if (itemMatches.length === 0) activeIndex = -1;
		else if (!options.preserveIndex || activeIndex < 0) activeIndex = 0;
		else activeIndex = Math.min(activeIndex, itemMatches.length - 1);
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
		itemMatches = [];
		activeIndex = -1;
		paintVersion += 1;
		clearHighlights();
		if (options.restoreFocus !== false && previousFocus?.isConnected) previousFocus.focus();
		previousFocus = null;
	}

	function setQuery(next: string) {
		query = next;
		rebuild({ scroll: true });
	}

	function move(direction: 1 | -1) {
		if (itemMatches.length === 0) return;
		activeIndex = (activeIndex + direction + itemMatches.length) % itemMatches.length;
		paintHighlights();
		void scrollActiveIntoView();
	}

	function observe() {
		const root = deps.getRoot();
		if (!root || typeof MutationObserver === 'undefined') return () => {};
		let timer: ReturnType<typeof setTimeout> | null = null;
		const observer = new MutationObserver(() => {
			if (timer) return;
			timer = setTimeout(() => {
				timer = null;
				// Store match list is authoritative; only refresh DOM highlights / expand.
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
			return itemMatches.length;
		},
		get activeIndex() {
			return activeIndex;
		},
		get activeTurnIndex() {
			return activeItemMatch()?.turnIndex ?? -1;
		},
		get activeItemId() {
			return activeItemMatch()?.itemId ?? null;
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
