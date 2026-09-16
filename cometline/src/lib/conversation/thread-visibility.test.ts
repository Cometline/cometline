import { describe, expect, it } from 'vitest';
import {
	firstAssistantInNormalList,
	hasVisibleThinkingBlock,
	selectFirstAssistantItem,
	showAssistantRow,
	showFirstTurnAvatarSlot,
	type ThreadVisibilityContext
} from './thread-visibility';
import type { ChatItem } from '$lib/stores/chat.svelte';
import {
	buildThinkingAttribution,
	type ThinkingAttribution
} from './thinking-attribution';

const emptyAttribution: ThinkingAttribution = {
	map: new Map(),
	toolIdsInBuffer: new Set(),
	subagentIdsInBuffer: new Set(),
	memoryIdsInBuffer: new Set(),
	errorIdsInBuffer: new Set()
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

describe('selectFirstAssistantItem', () => {
	it('unlocks first-turn activity when pending assistant has attributed tools and no reasoning/text', () => {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: ''
		};
		const tool: Extract<ChatItem, { type: 'tool' }> = {
			id: 't1',
			type: 'tool',
			toolId: 'tc-1',
			toolName: 'read_file',
			input: { path: 'README.md' },
			pending: true
		};
		const threadItems: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'read it' },
			assistant,
			tool
		];
		const attribution = buildThinkingAttribution(threadItems);
		expect(hasVisibleThinkingBlock('a1', threadItems, attribution)).toBe(true);
		expect(selectFirstAssistantItem(threadItems, attribution)?.id).toBe('a1');
		expect(
			showAssistantRow(
				assistant,
				ctx({
					threadItems,
					thinkingForAssistant: attribution,
					streamingAssistantId: 'a1',
					sessionStreaming: true,
					firstAssistantItem: assistant
				})
			)
		).toBe(true);
	});

	it('still selects an assistant that only has reasoning', () => {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: '',
			reasoning: { segments: [{ text: 'planning', pending: true }] }
		};
		const threadItems: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'hi' },
			assistant
		];
		const attribution = buildThinkingAttribution(threadItems);
		expect(selectFirstAssistantItem(threadItems, attribution)?.id).toBe('a1');
	});

	it('ignores empty follow-up assistants without attributed activity', () => {
		const first: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: 'done'
		};
		const followUp: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a2',
			type: 'assistant',
			text: ''
		};
		const threadItems: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'one' },
			first,
			{ id: 'u2', type: 'user', text: 'two' },
			followUp
		];
		const attribution = buildThinkingAttribution(threadItems);
		expect(selectFirstAssistantItem(threadItems, attribution)?.id).toBe('a1');
	});

	it('keeps tools grouped via attribution (no standalone qualification alone)', () => {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: 'a1',
			type: 'assistant',
			text: ''
		};
		const tool: Extract<ChatItem, { type: 'tool' }> = {
			id: 't1',
			type: 'tool',
			toolId: 'tc-1',
			toolName: 'list_dir',
			input: { path: '.' },
			pending: false,
			output: 'ok'
		};
		const threadItems: ChatItem[] = [assistant, tool];
		const attribution = buildThinkingAttribution(threadItems);
		expect(attribution.toolIdsInBuffer.has('t1')).toBe(true);
		expect(selectFirstAssistantItem(threadItems, attribution)?.id).toBe('a1');
	});
});
