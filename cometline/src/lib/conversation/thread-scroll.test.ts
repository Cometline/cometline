// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import {
	buildScrollKey,
	distanceToLatestBottom,
	followUpPinScrollMargin,
	isNearBottom,
	shouldShowJumpToBottom
} from './thread-scroll';
import type { ChatItem } from '$lib/stores/chat.svelte';

describe('buildScrollKey', () => {
	it('returns idle key when not streaming', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'hi' },
			{ id: 'a1', type: 'assistant', text: 'hello' }
		];
		expect(buildScrollKey(items, false)).toBe('idle:2:a1');
	});

	it('returns assistant stream key when streaming', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'hi' },
			{ id: 'a1', type: 'assistant', text: 'hel', pending: true }
		];
		expect(buildScrollKey(items, true)).toBe('stream:assistant:a1:3:0:false:true');
	});
});

describe('isNearBottom', () => {
	it('returns true when within threshold of bottom', () => {
		const el = {
			scrollHeight: 1000,
			scrollTop: 904,
			clientHeight: 96
		} as HTMLElement;
		expect(isNearBottom(el, 96)).toBe(true);
	});
});

describe('shouldShowJumpToBottom', () => {
	it('returns false when already near the bottom', () => {
		const el = {
			scrollHeight: 1000,
			scrollTop: 904,
			clientHeight: 96
		} as HTMLElement;
		expect(shouldShowJumpToBottom(el, null, 96)).toBe(false);
	});

	it('returns true when latest content is below the viewport', () => {
		const el = {
			scrollHeight: 1000,
			scrollTop: 0,
			clientHeight: 96
		} as HTMLElement;
		expect(shouldShowJumpToBottom(el, null, 96)).toBe(true);
	});

	it('ignores spacer height after the latest content sentinel', () => {
		const el = {
			scrollHeight: 1400,
			scrollTop: 0,
			clientHeight: 600,
			getBoundingClientRect: () => ({ bottom: 600 }) as DOMRect
		} as HTMLElement;
		const latest = {
			getBoundingClientRect: () => ({ bottom: 590 }) as DOMRect
		} as HTMLElement;

		expect(distanceToLatestBottom(el, latest)).toBe(-10);
		expect(shouldShowJumpToBottom(el, latest, 96)).toBe(false);
	});
});

describe('followUpPinScrollMargin', () => {
	it('uses the upper-third ratio when viewport height is known', () => {
		expect(followUpPinScrollMargin(800)).toBe(224);
	});

	it('falls back when viewport height is zero', () => {
		expect(followUpPinScrollMargin(0)).toBe(100);
	});
});
