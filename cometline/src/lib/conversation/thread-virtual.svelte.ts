import { untrack } from 'svelte';
import {
	THREAD_TURN_GAP,
	computeVirtualWindow,
	prefixOffsets,
	resolveTurnSizes,
	scrollDeltaForSizeChange,
	shouldSkipMegaMarkdownDuringHydration,
	virtualTurnEntriesWithForced
} from './thread-virtualizer';
import type { ThreadTurn } from './thread-turns';
import type { VirtualTurnEntry } from './thread-virtualizer';

export interface VisibleTurnEntry extends VirtualTurnEntry<ThreadTurn> {
	/**
	 * Mega turns skip Shiki only while `isInitialTranscriptPaint`.
	 * Always false after hydration — no scroll-gated defer (#157 UX).
	 */
	skipHydrationMarkdown: boolean;
}

export interface ThreadVirtualDeps {
	getSessionId: () => string;
	getThreadTurns: () => readonly ThreadTurn[];
	getScroller: () => HTMLDivElement | undefined;
	getViewportHeight: () => number;
	getActivePinnedUserId: () => string | null;
	getActiveTurnMinHeight: () => number;
	getLastUserId: () => string | null;
	getIsInitialTranscriptPaint: () => boolean;
	getFindOpen: () => boolean;
	getFindActiveTurnIndex: () => number;
	/** Re-run find jump when query/active match changes. */
	getFindJumpKey: () => string;
}

/**
 * Variable-height turn virtualization state + side effects.
 * Pure window math stays in thread-virtualizer.ts; this controller owns
 * measure cache, scrollTop sync, hydration rAF alignment, and find force-scroll.
 */
export function createThreadVirtual(deps: ThreadVirtualDeps) {
	let virtualScrollTop = $state(0);
	let measuredTurnHeights = $state.raw<Record<string, number>>({});
	let measureSessionId: string | null = null;

	const turnSizes = $derived(
		resolveTurnSizes(deps.getThreadTurns(), measuredTurnHeights, {
			activePinnedUserId: deps.getActivePinnedUserId(),
			activeTurnMinHeight: deps.getActiveTurnMinHeight()
		})
	);

	const virtualWindow = $derived(
		computeVirtualWindow(
			turnSizes,
			virtualScrollTop,
			deps.getViewportHeight() || deps.getScroller()?.clientHeight || 0
		)
	);

	const forcedTurnIndices = $derived.by(() => {
		const turns = deps.getThreadTurns();
		const pinnedId = deps.getActivePinnedUserId();
		const pinnedIndex = pinnedId ? turns.findIndex((turn) => turn.id === pinnedId) : -1;
		const lastUserId = deps.getLastUserId();
		const latestIndex = lastUserId ? turns.findIndex((turn) => turn.id === lastUserId) : -1;
		const findIndex = deps.getFindOpen() ? deps.getFindActiveTurnIndex() : -1;
		return [pinnedIndex, latestIndex, findIndex];
	});

	const visibleTurns = $derived.by((): VisibleTurnEntry[] => {
		const hydrating = deps.getIsInitialTranscriptPaint();
		const entries = virtualTurnEntriesWithForced(
			deps.getThreadTurns(),
			turnSizes,
			virtualWindow,
			forcedTurnIndices
		);
		return entries.map((entry) => ({
			...entry,
			skipHydrationMarkdown: shouldSkipMegaMarkdownDuringHydration(entry.item, hydrating)
		}));
	});

	function setScrollTop(top: number) {
		virtualScrollTop = top;
	}

	function syncFromScroller() {
		virtualScrollTop = deps.getScroller()?.scrollTop ?? 0;
	}

	function onScroll() {
		syncFromScroller();
	}

	$effect(() => {
		const next = deps.getSessionId();
		if (next === measureSessionId) return;
		measureSessionId = next;
		measuredTurnHeights = {};
		virtualScrollTop = 0;
	});

	// Hydration / stick-to-bottom sets scrollTop in rAF; keep the virtual window
	// aligned even if a programmatic assignment does not emit `scroll`.
	$effect(() => {
		if (!deps.getIsInitialTranscriptPaint()) return;
		const scroller = deps.getScroller();
		if (!scroller) return;
		let frame = 0;
		const sync = () => {
			virtualScrollTop = deps.getScroller()?.scrollTop ?? 0;
			frame = requestAnimationFrame(sync);
		};
		frame = requestAnimationFrame(sync);
		return () => cancelAnimationFrame(frame);
	});

	// Jump the virtual scroller to the active find turn so it enters the window /
	// forced-mount set without remounting the whole transcript.
	$effect(() => {
		if (!deps.getFindOpen()) return;
		const turnIndex = deps.getFindActiveTurnIndex();
		void deps.getFindJumpKey();
		const scroller = deps.getScroller();
		if (turnIndex < 0 || !scroller) return;
		const sizes = untrack(() => turnSizes);
		const offsets = prefixOffsets(sizes);
		const top = offsets[turnIndex] ?? 0;
		const nextTop = Math.max(0, top - scroller.clientHeight * 0.25);
		scroller.scrollTo({ top: nextTop, behavior: 'auto' });
		virtualScrollTop = scroller.scrollTop;
	});

	function onTurnMeasured(turnId: string, height: number) {
		const next = Math.ceil(height);
		if (next <= 0) return;
		const prev = measuredTurnHeights[turnId];
		if (prev === next) return;
		const turns = deps.getThreadTurns();
		const index = turns.findIndex((turn) => turn.id === turnId);
		let itemOffset = 0;
		if (index > 0) {
			for (let i = 0; i < index; i++) {
				itemOffset += (turnSizes[i] ?? 0) + THREAD_TURN_GAP;
			}
		}
		const scroller = deps.getScroller();
		const delta = scrollDeltaForSizeChange(
			itemOffset,
			scroller?.scrollTop ?? virtualScrollTop,
			prev ?? turnSizes[index] ?? next,
			next
		);
		measuredTurnHeights = { ...measuredTurnHeights, [turnId]: next };
		if (delta !== 0 && scroller) {
			scroller.scrollTop += delta;
			virtualScrollTop = scroller.scrollTop;
		}
	}

	function measureTurnHeight(
		node: HTMLElement,
		params: { id: string; onMeasure: (id: string, height: number) => void }
	) {
		const notify = () => params.onMeasure(params.id, node.offsetHeight);
		notify();
		if (typeof ResizeObserver === 'undefined') {
			return {
				update(next: { id: string; onMeasure: (id: string, height: number) => void }) {
					params = next;
					notify();
				}
			};
		}
		const observer = new ResizeObserver(() => notify());
		observer.observe(node);
		return {
			update(next: { id: string; onMeasure: (id: string, height: number) => void }) {
				params = next;
				notify();
			},
			destroy() {
				observer.disconnect();
			}
		};
	}

	return {
		get virtualScrollTop() {
			return virtualScrollTop;
		},
		get virtualWindow() {
			return virtualWindow;
		},
		get visibleTurns() {
			return visibleTurns;
		},
		setScrollTop,
		syncFromScroller,
		onScroll,
		onTurnMeasured,
		measureTurnHeight
	};
}

export type ThreadVirtualController = ReturnType<typeof createThreadVirtual>;
