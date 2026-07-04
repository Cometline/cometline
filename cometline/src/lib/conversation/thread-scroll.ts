import type { ChatItem } from '$lib/stores/chat.svelte';
import { anyReasoningPending, reasoningTextLength } from './reasoning';

export const SCROLL_PIN_THRESHOLD = 96;
export const USER_SEND_TOP_OFFSET = 24;
/** Upper-third pin for follow-up turns (turn 2+), applied via scroll-margin-top. */
export const USER_PIN_RATIO_FOLLOWUP = 0.28;
export const USER_PIN_OFFSET_FALLBACK = 100;
export const TURN_BOTTOM_CLEARANCE = 96;

export function isNearBottom(element: HTMLElement, threshold = SCROLL_PIN_THRESHOLD) {
	return element.scrollHeight - element.scrollTop - element.clientHeight <= threshold;
}

export function distanceToLatestBottom(element: HTMLElement, latest?: HTMLElement | null) {
	if (!latest) return element.scrollHeight - element.scrollTop - element.clientHeight;
	return latest.getBoundingClientRect().bottom - element.getBoundingClientRect().bottom;
}

export function shouldShowJumpToBottom(
	element: HTMLElement,
	latest?: HTMLElement | null,
	threshold = SCROLL_PIN_THRESHOLD
) {
	const overflowing = element.scrollHeight - element.clientHeight > threshold;
	return overflowing && distanceToLatestBottom(element, latest) > threshold;
}

export function buildScrollKey(items: readonly ChatItem[], sessionStreaming: boolean) {
	if (!sessionStreaming) {
		const last = items.at(-1);
		return `idle:${items.length}:${last?.id ?? ''}`;
	}
	for (let i = items.length - 1; i >= 0; i--) {
		const item = items[i];
		if (item.type === 'assistant') {
			return `stream:assistant:${item.id}:${item.text.length}:${reasoningTextLength(item)}:${anyReasoningPending(item)}:${item.pending ?? false}`;
		}
		if (item.type === 'tool') {
			return `stream:tool:${item.id}:${item.output?.length ?? 0}:${item.pending ?? false}`;
		}
	}
	return `stream:empty:${items.length}`;
}

export function followUpPinScrollMargin(viewportHeight: number) {
	return viewportHeight > 0
		? Math.round(viewportHeight * USER_PIN_RATIO_FOLLOWUP)
		: USER_PIN_OFFSET_FALLBACK;
}

export function countUserMessages(items: readonly ChatItem[]) {
	return items.reduce((count, item) => (item.type === 'user' ? count + 1 : count), 0);
}
