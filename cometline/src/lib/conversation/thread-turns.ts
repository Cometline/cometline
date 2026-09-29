import type { ChatItem } from '$lib/stores/chat.svelte';
import { TURN_BOTTOM_CLEARANCE } from './thread-scroll';

export interface ThreadTurnItem {
	item: ChatItem;
	index: number;
}

export type ThreadUserItem = Extract<ChatItem, { type: 'user' }>;

export interface ThreadTurn {
	id: string;
	/**
	 * Anchoring user message. Null for a synthetic orphan/continued turn when the
	 * transcript page starts mid-turn (leading assistant/tool/reasoning before any user).
	 */
	user: ThreadUserItem | null;
	/** Index of `user` in the flat items list; -1 when `orphan`. */
	userIndex: number;
	items: ThreadTurnItem[];
	/** True when this turn has no real user anchor (page started mid-turn). */
	orphan?: boolean;
}

/** True when the flat transcript starts with non-user items (mid-turn page orphan). */
export function transcriptHasLeadingOrphans(items: readonly ChatItem[]): boolean {
	return items.length > 0 && items[0].type !== 'user';
}

/** Group a flat transcript into user-anchored turns (plus a leading orphan turn if needed). */
export function groupThreadItemsIntoTurns(items: readonly ChatItem[]): ThreadTurn[] {
	const turns: ThreadTurn[] = [];
	let current: ThreadTurn | null = null;

	for (let index = 0; index < items.length; index++) {
		const item = items[index];
		if (item.type === 'user') {
			current = { id: item.id, user: item, userIndex: index, items: [] };
			turns.push(current);
			continue;
		}
		if (!current) {
			// Synthetic continued/orphan turn so leading assistant/tool body cannot vanish.
			current = {
				id: `orphan:${item.id}`,
				user: null,
				userIndex: -1,
				items: [],
				orphan: true
			};
			turns.push(current);
		}
		current.items.push({ item, index });
	}

	return turns;
}

/** Min-height for the active turn canvas (follow-up turns only). */
export function activeTurnMinHeight(viewportHeight: number, clearance = TURN_BOTTOM_CLEARANCE) {
	if (viewportHeight <= 0) return 0;
	return Math.max(0, viewportHeight - clearance);
}
