import type { ChatItem } from '$lib/stores/chat.svelte';
import type { ThreadTurn } from './thread-turns';

export const SESSION_FIND_MATCH_HIGHLIGHT = 'session-find-match';
export const SESSION_FIND_ACTIVE_HIGHLIGHT = 'session-find-active';

type CharacterPosition = {
	node: Text;
	offset: number;
};

export type SessionFindMatch = {
	range: Range;
	root: HTMLElement;
};

/** Store-backed match used while the thread is virtualized (no full DOM mount). */
export type SessionItemFindMatch = {
	itemId: string;
	turnId: string;
	turnIndex: number;
	/** 0-based occurrence of the needle within this item's searchable text. */
	occurrenceInItem: number;
};

const BLOCK_SELECTOR = 'p,li,pre,blockquote,h1,h2,h3,h4,h5,h6,td,th,div';
const IGNORED_SELECTOR = 'button,[aria-hidden="true"],[hidden]';

function searchableTextNodes(root: HTMLElement): Text[] {
	const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
		acceptNode(node) {
			const parent = node.parentElement;
			if (!parent || parent.closest(IGNORED_SELECTOR)) return NodeFilter.FILTER_REJECT;
			return node.textContent ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
		}
	});
	const nodes: Text[] = [];
	let node = walker.nextNode();
	while (node) {
		nodes.push(node as Text);
		node = walker.nextNode();
	}
	return nodes;
}

function appendSpace(
	text: string[],
	positions: CharacterPosition[],
	node: Text,
	offset: number
) {
	if (text.length === 0 || text.at(-1) === ' ') return;
	text.push(' ');
	positions.push({ node, offset });
}

function indexSearchRoot(root: HTMLElement) {
	const text: string[] = [];
	const positions: CharacterPosition[] = [];
	let previousBlock: Element | null = null;

	for (const node of searchableTextNodes(root)) {
		const block = node.parentElement?.closest(BLOCK_SELECTOR) ?? root;
		if (previousBlock && block !== previousBlock) appendSpace(text, positions, node, 0);
		previousBlock = block;
		const value = node.data;
		for (let offset = 0; offset < value.length; offset += 1) {
			const char = value[offset];
			if (/\s/u.test(char)) {
				appendSpace(text, positions, node, offset);
				continue;
			}
			text.push(char);
			positions.push({ node, offset });
		}
	}

	return { text: text.join('').trimEnd(), positions };
}

export function normalizeQuery(query: string): string {
	return query.replace(/\s+/gu, ' ').trim();
}

export function escapeRegExp(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function rangeForOffsets(positions: CharacterPosition[], start: number, end: number): Range | null {
	const first = positions[start];
	const last = positions[end - 1];
	if (!first || !last) return null;
	const range = document.createRange();
	range.setStart(first.node, first.offset);
	range.setEnd(last.node, last.offset + 1);
	return range;
}

export function findSessionTextMatches(root: HTMLElement, query: string): SessionFindMatch[] {
	const needle = normalizeQuery(query);
	if (!needle) return [];
	const pattern = new RegExp(escapeRegExp(needle), 'giu');
	const matches: SessionFindMatch[] = [];
	const searchRoots = root.querySelectorAll<HTMLElement>('[data-session-find-text]');

	for (const searchRoot of searchRoots) {
		if (searchRoot.closest('[aria-hidden="true"]')) continue;
		const indexed = indexSearchRoot(searchRoot);
		for (const match of indexed.text.matchAll(pattern)) {
			const start = match.index;
			const range = rangeForOffsets(indexed.positions, start, start + match[0].length);
			if (range) matches.push({ range, root: searchRoot });
		}
	}
	return matches;
}

function searchableItemText(item: ChatItem): string | null {
	if (item.type === 'user' || item.type === 'assistant') {
		return item.text ?? '';
	}
	return null;
}

/**
 * Find matches in transcript plain text (user / assistant bodies) without mounting
 * offscreen turns. Occurrence order matches transcript order.
 */
export function findSessionItemMatches(
	turns: readonly ThreadTurn[],
	query: string
): SessionItemFindMatch[] {
	const needle = normalizeQuery(query);
	if (!needle) return [];
	const pattern = new RegExp(escapeRegExp(needle), 'giu');
	const matches: SessionItemFindMatch[] = [];

	for (let turnIndex = 0; turnIndex < turns.length; turnIndex++) {
		const turn = turns[turnIndex];
		const items: ChatItem[] = turn.user
			? [turn.user, ...turn.items.map((entry) => entry.item)]
			: turn.items.map((entry) => entry.item);
		for (const item of items) {
			const raw = searchableItemText(item);
			if (raw === null) continue;
			const text = normalizeQuery(raw);
			if (!text) continue;
			let occurrenceInItem = 0;
			for (const match of text.matchAll(pattern)) {
				void match;
				matches.push({
					itemId: item.id,
					turnId: turn.id,
					turnIndex,
					occurrenceInItem
				});
				occurrenceInItem += 1;
			}
		}
	}

	return matches;
}
