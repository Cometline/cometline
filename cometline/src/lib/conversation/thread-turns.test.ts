import { describe, expect, it } from 'vitest';
import {
	activeTurnMinHeight,
	groupThreadItemsIntoTurns,
	transcriptHasLeadingOrphans
} from './thread-turns';
import type { ChatItem } from '$lib/stores/chat.svelte';

describe('groupThreadItemsIntoTurns', () => {
	it('groups follow-up items under the preceding user message', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'first' },
			{ id: 'a1', type: 'assistant', text: 'reply' },
			{ id: 't1', type: 'tool', toolName: 'read', input: {}, output: 'ok' },
			{ id: 'u2', type: 'user', text: 'second' },
			{ id: 'a2', type: 'assistant', text: 'again' }
		];

		const turns = groupThreadItemsIntoTurns(items);

		expect(turns).toHaveLength(2);
		expect(turns[0].id).toBe('u1');
		expect(turns[0].items.map((entry) => entry.item.id)).toEqual(['a1', 't1']);
		expect(turns[1].id).toBe('u2');
		expect(turns[1].items.map((entry) => entry.item.id)).toEqual(['a2']);
	});

	it('returns an empty list for an empty transcript', () => {
		expect(groupThreadItemsIntoTurns([])).toEqual([]);
	});

	it('keeps leading mega body in a synthetic orphan turn before hi', () => {
		const mega = 'MEGA_BODY_' + 'x'.repeat(200);
		const items: ChatItem[] = [
			{ id: 'a-mega', type: 'assistant', text: mega },
			{ id: 't1', type: 'tool', toolName: 'bash', input: { cmd: 'ls' }, output: 'ok' },
			{ id: 'a-mid', type: 'assistant', text: 'still working' },
			{ id: 'u-hi', type: 'user', text: 'hi' },
			{ id: 'a-hello', type: 'assistant', text: 'hello' }
		];

		const turns = groupThreadItemsIntoTurns(items);

		expect(turns).toHaveLength(2);
		expect(turns[0].orphan).toBe(true);
		expect(turns[0].user).toBeNull();
		expect(turns[0].id).toBe('orphan:a-mega');
		expect(turns[0].items.map((entry) => entry.item.id)).toEqual(['a-mega', 't1', 'a-mid']);
		expect(
			turns[0].items.some(
				(entry) => entry.item.type === 'assistant' && entry.item.text.includes('MEGA_BODY_')
			)
		).toBe(true);
		expect(turns[1].id).toBe('u-hi');
		expect(turns[1].user?.text).toBe('hi');
		expect(turns[1].items.map((entry) => entry.item.id)).toEqual(['a-hello']);
	});

	it('emits a single orphan turn when the page has no user at all', () => {
		const items: ChatItem[] = [
			{ id: 'a1', type: 'assistant', text: 'partial mega' },
			{ id: 't1', type: 'tool', toolName: 'read', input: {}, output: 'x' }
		];
		const turns = groupThreadItemsIntoTurns(items);
		expect(turns).toHaveLength(1);
		expect(turns[0].orphan).toBe(true);
		expect(turns[0].items).toHaveLength(2);
	});
});

describe('transcriptHasLeadingOrphans', () => {
	it('is false for empty or user-leading transcripts', () => {
		expect(transcriptHasLeadingOrphans([])).toBe(false);
		expect(transcriptHasLeadingOrphans([{ id: 'u1', type: 'user', text: 'hi' }])).toBe(false);
	});

	it('is true when the first item is non-user', () => {
		expect(
			transcriptHasLeadingOrphans([
				{ id: 'a1', type: 'assistant', text: 'mega' },
				{ id: 'u1', type: 'user', text: 'hi' }
			])
		).toBe(true);
	});
});

describe('activeTurnMinHeight', () => {
	it('subtracts clearance from the viewport height', () => {
		expect(activeTurnMinHeight(600)).toBe(504);
	});

	it('returns zero when the viewport is unknown', () => {
		expect(activeTurnMinHeight(0)).toBe(0);
	});
});
