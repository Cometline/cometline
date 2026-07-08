import { describe, expect, it } from 'vitest';
import { itemsFromTranscript } from './chat-transcript';

describe('itemsFromTranscript', () => {
	it('maps user messages from transcript rows', () => {
		const items = itemsFromTranscript([
			{ type: 'user', text: 'Hi' },
			{ type: 'assistant', text: 'Hello' }
		]);
		expect(items).toHaveLength(2);
		expect(items[0]).toMatchObject({ type: 'user', text: 'Hi' });
		expect(items[1]).toMatchObject({ type: 'assistant', text: 'Hello' });
	});

	it('keeps transcript errors attached to the assistant activity block', () => {
		const items = itemsFromTranscript([
			{ type: 'user', text: 'Run this' },
			{ type: 'tool', tool_name: 'read_file', tool_input: { path: 'README.md' } },
			{ type: 'error', text: 'provider failed' }
		]);

		expect(items.map((item) => item.type)).toEqual(['user', 'assistant', 'tool', 'error']);
		expect(items[1]).toMatchObject({ type: 'assistant', text: '' });
		expect(items[3]).toMatchObject({ type: 'error', text: 'provider failed' });
	});
});
