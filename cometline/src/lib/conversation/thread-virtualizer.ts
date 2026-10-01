import type { ChatItem } from '$lib/stores/chat.svelte';
import type { ThreadTurn } from './thread-turns';

/** Gap between `.thread-turn` siblings (ChatThread mobile-first CSS). */
export const THREAD_TURN_GAP = 14;

/** Extra turns mounted above/below the viewport. */
export const THREAD_VIRTUAL_OVERSCAN = 2;

/** Floor for empty/short turns so placeholders never collapse to zero. */
export const THREAD_TURN_ESTIMATE_MIN = 120;

/**
 * Soft ceiling so a single giant estimate cannot explode total height.
 * Raised well above pre-PR2.5 2400 so mega / CJK replies do not leave lasting
 * blank holes under the absolute spacer before measure settles.
 */
export const THREAD_TURN_ESTIMATE_MAX = 48_000;

/**
 * Assistant bodies at or above this length skip full AssistantMarkdown / Shiki
 * only while `isInitialTranscriptPaint` (open cost). After hydration clears,
 * the same component instance runs full markdown once — no scroll-gated defer.
 */
export const THREAD_MEGA_ASSISTANT_CHARS = 4_000;

/**
 * Wall-clock failsafe for clearing isInitialTranscriptPaint (ms).
 * Armed once when hydration begins; must survive settle-effect restarts from
 * threadItems / sync churn (that was the #157 miss).
 */
export const THREAD_HYDRATION_FAILSAFE_MS = 400;

const USER_BASE = 72;
const ASSISTANT_BASE = 56;
const TOOL_ROW = 44;
const SUBAGENT_ROW = 48;
const MEMORY_ROW = 40;
const ERROR_ROW = 40;
const STATUS_ROW = 32;
const CHARS_PER_LINE = 88;
const LINE_HEIGHT = 22;

/** Wide / CJK code points count as ~2 Latin columns for line estimates. */
const WIDE_CHAR_RE = /[ᄀ-ᅟ⺀-꓏가-힣豈-﫿︐-︙︰-﹯＀-｠￠-￦]/u;

export interface VirtualWindow {
	start: number;
	end: number;
	offset: number;
	totalHeight: number;
}

export interface VirtualTurnEntry<T = ThreadTurn> {
	item: T;
	index: number;
	offset: number;
}

/** Visual column count: CJK / fullwidth ≈ 2, else 1. */
export function visualColumnCount(text: string): number {
	let cols = 0;
	for (const ch of text) {
		cols += WIDE_CHAR_RE.test(ch) ? 2 : 1;
	}
	return cols;
}

/** Rough block height from plaintext length (markdown/layout TBD until measured). */
function textBlockHeight(text: string | undefined, base: number): number {
	const raw = text?.trim() ?? '';
	if (raw.length === 0) return base;
	const lines = Math.ceil(visualColumnCount(raw) / CHARS_PER_LINE);
	return base + lines * LINE_HEIGHT;
}

function estimateItemHeight(item: ChatItem): number {
	switch (item.type) {
		case 'user':
			return (
				textBlockHeight(item.text, USER_BASE) +
				(item.images?.length ? Math.min(320, item.images.length * 120) : 0) +
				(item.contexts?.length ? 28 : 0)
			);
		case 'assistant':
			return (
				textBlockHeight(item.text, ASSISTANT_BASE) +
				(item.images?.length ? Math.min(360, item.images.length * 140) : 0) +
				(item.reasoning ? 52 : 0)
			);
		case 'tool':
			return TOOL_ROW + (item.pending ? 8 : 0);
		case 'subagent':
			return SUBAGENT_ROW;
		case 'memory':
			return MEMORY_ROW;
		case 'error':
			return ERROR_ROW;
		case 'status':
			return STATUS_ROW;
		default:
			return THREAD_TURN_ESTIMATE_MIN;
	}
}

/** Estimate a turn's laid-out height before the row is measured. Prefers slight overestimate. */
export function estimateTurnHeight(turn: ThreadTurn): number {
	let height = turn.user ? estimateItemHeight(turn.user) : 0;
	for (const entry of turn.items) {
		height += estimateItemHeight(entry.item);
	}
	// Inter-row gaps inside a turn (user + follow-ups share the turn's flex gap).
	const rowCount = (turn.user ? 1 : 0) + turn.items.length;
	if (rowCount > 1) height += (rowCount - 1) * THREAD_TURN_GAP;
	return Math.min(
		THREAD_TURN_ESTIMATE_MAX,
		Math.max(THREAD_TURN_ESTIMATE_MIN, Math.ceil(height))
	);
}

/** True when any assistant body in the turn is large enough to defer Shiki. */
export function isOversizedAssistantText(text: string | undefined): boolean {
	return (text?.length ?? 0) >= THREAD_MEGA_ASSISTANT_CHARS;
}

export function turnHasOversizedAssistant(turn: ThreadTurn): boolean {
	return turn.items.some(
		(entry) => entry.item.type === 'assistant' && isOversizedAssistantText(entry.item.text)
	);
}

/**
 * Mega assistant turns skip full markdown/Shiki **only** while hydrating.
 * The moment hydration clears, callers must run full markdown on the same
 * mounted instance — no scroll-gated plaintext defer after interactive (#157 UX).
 */
export function shouldSkipMegaMarkdownDuringHydration(
	turn: ThreadTurn,
	isHydrating: boolean
): boolean {
	return isHydrating && turnHasOversizedAssistant(turn);
}

/**
 * Resolve per-turn sizes: measured wins, else estimate. Active follow-up canvas
 * min-height is applied so pin layout matches the non-virtual path.
 */
export function resolveTurnSizes(
	turns: readonly ThreadTurn[],
	measuredById: Readonly<Record<string, number>>,
	options: {
		activePinnedUserId?: string | null;
		activeTurnMinHeight?: number;
	} = {}
): number[] {
	const { activePinnedUserId = null, activeTurnMinHeight = 0 } = options;
	return turns.map((turn) => {
		const measured = measuredById[turn.id];
		let size =
			typeof measured === 'number' && measured > 0 ? measured : estimateTurnHeight(turn);
		if (activePinnedUserId && turn.id === activePinnedUserId && activeTurnMinHeight > 0) {
			size = Math.max(size, activeTurnMinHeight);
		}
		return size;
	});
}

/** Start offset for each turn; `offsets[count]` is totalHeight. */
export function prefixOffsets(sizes: readonly number[], gap = THREAD_TURN_GAP): number[] {
	const offsets = new Array<number>(sizes.length + 1);
	offsets[0] = 0;
	for (let i = 0; i < sizes.length; i++) {
		offsets[i + 1] = offsets[i] + sizes[i] + (i < sizes.length - 1 ? gap : 0);
	}
	return offsets;
}

export function totalHeightFromSizes(sizes: readonly number[], gap = THREAD_TURN_GAP): number {
	if (sizes.length === 0) return 0;
	let total = 0;
	for (let i = 0; i < sizes.length; i++) {
		total += sizes[i];
		if (i < sizes.length - 1) total += gap;
	}
	return total;
}

function findStartIndex(
	offsets: readonly number[],
	sizes: readonly number[],
	scrollTop: number
): number {
	const n = sizes.length;
	if (n === 0) return 0;
	let lo = 0;
	let hi = n - 1;
	let start = 0;
	while (lo <= hi) {
		const mid = (lo + hi) >> 1;
		const end = offsets[mid] + sizes[mid];
		if (end <= scrollTop) {
			lo = mid + 1;
			start = lo;
		} else {
			hi = mid - 1;
		}
	}
	return Math.min(start, n);
}

/**
 * Variable-height window: mount `[start, end)` plus overscan. `offset` is the
 * absolute top of `start` (for translate/absolute positioning).
 */
export function computeVirtualWindow(
	sizes: readonly number[],
	scrollTop: number,
	viewportHeight: number,
	overscan = THREAD_VIRTUAL_OVERSCAN,
	gap = THREAD_TURN_GAP
): VirtualWindow {
	const count = sizes.length;
	if (count === 0) {
		return { start: 0, end: 0, offset: 0, totalHeight: 0 };
	}

	const offsets = prefixOffsets(sizes, gap);
	const totalHeight = offsets[count];
	const safeScroll = Math.max(0, scrollTop);
	const safeViewport = Math.max(0, viewportHeight);

	if (safeViewport <= 0) {
		// Prefer the tail so stick-to-bottom hydration measures real bottom rows first.
		const start = Math.max(0, count - Math.max(overscan * 2 + 3, 1));
		return { start, end: count, offset: offsets[start], totalHeight };
	}

	const viewEnd = safeScroll + safeViewport;
	let start = findStartIndex(offsets, sizes, safeScroll);
	let end = start;
	while (end < count && offsets[end] < viewEnd) {
		end += 1;
	}
	if (end === start && start < count) end = start + 1;

	start = Math.max(0, start - overscan);
	end = Math.min(count, end + overscan);

	return {
		start,
		end,
		offset: offsets[start],
		totalHeight
	};
}

export function virtualTurnEntries<T>(
	items: readonly T[],
	sizes: readonly number[],
	window: VirtualWindow,
	gap = THREAD_TURN_GAP
): VirtualTurnEntry<T>[] {
	const offsets = prefixOffsets(sizes, gap);
	const entries: VirtualTurnEntry<T>[] = [];
	for (let index = window.start; index < window.end; index++) {
		entries.push({ item: items[index], index, offset: offsets[index] });
	}
	return entries;
}

/**
 * When a measured height changes for a row above the viewport, shift scrollTop
 * by the delta so content under the user's eyes does not jump.
 */
export function scrollDeltaForSizeChange(
	itemOffset: number,
	scrollTop: number,
	prevSize: number,
	nextSize: number
): number {
	if (nextSize === prevSize) return 0;
	if (itemOffset >= scrollTop) return 0;
	return nextSize - prevSize;
}

/**
 * Build absolute-positioned entries for the viewport window, plus any forced
 * indices that fall outside it (latest sentinel / active pin). Forced extras
 * stay individually mounted — the contiguous window is not expanded to span
 * the gap (that would remount the entire transcript).
 */
export function virtualTurnEntriesWithForced<T>(
	items: readonly T[],
	sizes: readonly number[],
	window: VirtualWindow,
	forcedIndices: readonly number[],
	gap = THREAD_TURN_GAP
): VirtualTurnEntry<T>[] {
	const offsets = prefixOffsets(sizes, gap);
	const indexSet = new Set<number>();
	for (let index = window.start; index < window.end; index++) indexSet.add(index);
	for (const index of forcedIndices) {
		if (index >= 0 && index < items.length) indexSet.add(index);
	}
	return [...indexSet]
		.sort((a, b) => a - b)
		.map((index) => ({ item: items[index], index, offset: offsets[index] }));
}

/**
 * Keep the viewport anchored when older rows are prepended above the current
 * window. Without this, scrollTop stays put while content height grows upward
 * and the user sees a jump / blank hole.
 */
export function scrollTopAfterPrepend(
	prevScrollTop: number,
	prevScrollHeight: number,
	nextScrollHeight: number
): number {
	const delta = nextScrollHeight - prevScrollHeight;
	if (delta <= 0) return prevScrollTop;
	return Math.max(0, prevScrollTop + delta);
}
