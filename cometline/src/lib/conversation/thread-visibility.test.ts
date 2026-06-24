import { describe, expect, it } from 'vitest';
import {
	firstAssistantInNormalList,
	hasVisibleThinkingBlock,
	showAssistantRow,
	showFirstTurnAvatarSlot,
	type ThreadVisibilityContext
} from './thread-visibility';
import type { ChatItem } from '$lib/stores/chat.svelte';
import type { ThinkingAttribution } from './thinking-attribution';

const emptyAttribution: ThinkingAttribution = {
	map: new Map(),
	toolIdsInBuffer: new Set(),
	subagentIdsInBuffer: new Set(),
	memoryIdsInBuffer: new Set()
};

function ctx(overrides: Partial<ThreadVisibilityContext> = {}): ThreadVisibilityContext {
	return {
		threadItems: [],
		thinkingForAssistant: emptyAttribution,
		streamingAssistantId: null,
		sessionStreaming: false,
		awaitingFirstAssistant: false,
		firstTurnFlightDone: false,
		firstTurnHandoffPending: false,
		firstUserId: null,
		firstAssistantRowId: null,
		firstAssistantItem: undefined,
		...overrides
	};
}

describe('showAssistantRow', () => {
	it('returns true when assistant has visible text', () => {
		const item: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'hello'
		};
		expect(showAssistantRow(item, ctx({ threadItems: [item] }))).toBe(true);
	});

	it('returns true while streaming even without text yet', () => {
		const item: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: ''
		};
		expect(
			showAssistantRow(
				item,
				ctx({
					threadItems: [item],
					streamingAssistantId: 'a1',
					sessionStreaming: true
				})
			)
		).toBe(true);
	});
});

describe('showFirstTurnAvatarSlot', () => {
	it('returns true during handoff pending', () => {
		expect(
			showFirstTurnAvatarSlot(ctx({ firstUserId: 'u1', firstTurnHandoffPending: true }))
		).toBe(true);
	});

	it('returns false once first turn is done and assistant row is visible', () => {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'done'
		};
		expect(
			showFirstTurnAvatarSlot(
				ctx({
					firstUserId: 'u1',
					awaitingFirstAssistant: true,
					firstTurnFlightDone: true,
					firstAssistantItem: assistant,
					threadItems: [assistant]
				})
			)
		).toBe(false);
	});
});

describe('firstAssistantInNormalList', () => {
	it('excludes first assistant while awaiting first assistant slot', () => {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: ''
		};
		expect(
			firstAssistantInNormalList(
				assistant,
				ctx({
					firstUserId: 'u1',
					firstAssistantRowId: 'a1',
					awaitingFirstAssistant: true,
					firstTurnFlightDone: false
				})
			)
		).toBe(false);
	});
});

describe('hasVisibleThinkingBlock', () => {
	it('returns false when timeline is empty', () => {
		expect(hasVisibleThinkingBlock('a1', [], emptyAttribution)).toBe(false);
	});
});
