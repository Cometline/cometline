import type { ChatItem } from '$lib/stores/chat.svelte';

export const SCROLL_PIN_THRESHOLD = 96;
export const USER_SEND_TOP_OFFSET = 24;
/** Upper-third pin for follow-up turns (turn 2+), applied via scroll-margin-top. */
export const USER_PIN_RATIO_FOLLOWUP = 0.28;
export const USER_PIN_OFFSET_FALLBACK = 100;
export const TURN_BOTTOM_CLEARANCE = 96;

export function isNearBottom(element: HTMLElement, threshold = SCROLL_PIN_THRESHOLD) {
	return element.scrollHeight - element.scrollTop - element.clientHeight <= threshold;
}

export function followUpPinScrollMargin(viewportHeight: number) {
	return viewportHeight > 0
		? Math.round(viewportHeight * USER_PIN_RATIO_FOLLOWUP)
		: USER_PIN_OFFSET_FALLBACK;
}

export function countUserMessages(items: readonly ChatItem[]) {
	return items.reduce((count, item) => (item.type === 'user' ? count + 1 : count), 0);
}
