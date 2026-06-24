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
});
