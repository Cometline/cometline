import { describe, expect, it } from 'vitest';
import { speakerFor, startsSpeakerRun, timelineEntryKey } from './thread-view-helpers';
import type { ChatItem } from '$lib/stores/chat.svelte';

describe('speakerFor', () => {
	it('maps item types to speakers', () => {
		expect(speakerFor({ id: 'u1', type: 'user', text: 'hi' })).toBe('user');
		expect(speakerFor({ id: 'a1', type: 'assistant', text: '' })).toBe('assistant');
		expect(speakerFor({ id: 't1', type: 'tool', toolName: 'x', input: '', output: '' })).toBe(
			'assistant'
		);
		expect(speakerFor(undefined)).toBeNull();
		expect(speakerFor({ id: 's1', type: 'status', text: 'ok' })).toBeNull();
	});
});

describe('startsSpeakerRun', () => {
	const items: ChatItem[] = [
		{ id: 'u1', type: 'user', text: 'hi' },
		{ id: 'a1', type: 'assistant', text: 'hello' },
		{ id: 't1', type: 'tool', toolName: 'grep', input: '', output: '' }
	];

	it('returns true for the first speaker item', () => {
		expect(startsSpeakerRun(items, 0, 'user')).toBe(true);
	});

	it('returns false for continuation items from the same speaker', () => {
		expect(startsSpeakerRun(items, 2, 'assistant')).toBe(false);
	});

	it('returns true when the speaker changes', () => {
		expect(startsSpeakerRun(items, 1, 'assistant')).toBe(true);
	});
});

describe('timelineEntryKey', () => {
	it('keys reasoning segments by index', () => {
		expect(timelineEntryKey({ kind: 'reasoning', segmentIndex: 2, text: 'thinking' })).toBe(
			'reasoning-2'
		);
	});

	it('keys tools and subagents by id', () => {
		expect(
			timelineEntryKey({
				kind: 'tool',
				tool: { id: 't1', type: 'tool', toolName: 'grep', input: '', output: '' }
			})
		).toBe('tool-t1');
	});
});
