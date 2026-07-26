import { describe, expect, it } from 'vitest';
import type { ChatItem } from '$lib/types';
import {
	assistantResponseSource,
	buildAssistantResponseContext,
	normalizeAssistantSelection
} from './assistant-response-context';

const items: ChatItem[] = [
	{ id: 'user-1', type: 'user', text: 'Question one' },
	{ id: 'assistant-1', type: 'assistant', text: 'Answer one' },
	{ id: 'tool-1', type: 'tool', toolName: 'read_file', input: {}, pending: false },
	{ id: 'user-2', type: 'user', text: 'Question two' },
	{ id: 'assistant-2', type: 'assistant', text: 'Answer two' }
];

describe('assistant response context', () => {
	it('uses a stable assistant-only ordinal for the source', () => {
		expect(assistantResponseSource('session-1', 'assistant-2', items)).toBe(
			'assistant-response://session-1/2'
		);
	});

	it('normalizes the chip preview while preserving selected content', () => {
		const context = buildAssistantResponseContext({
			sessionId: 'session-1',
			itemId: 'assistant-1',
			items,
			selectedText: '  First line\n\n second   line  '
		});

		expect(context).toEqual({
			kind: 'message',
			title: 'First line second line',
			source: 'assistant-response://session-1/1',
			content: 'First line\n\n second   line'
		});
		expect(normalizeAssistantSelection(' one\n two ')).toBe('one two');
	});

	it('rejects empty selections and missing assistant items', () => {
		expect(
			buildAssistantResponseContext({
				sessionId: 'session-1',
				itemId: 'assistant-1',
				items,
				selectedText: '   '
			})
		).toBeNull();
		expect(assistantResponseSource('session-1', 'missing', items)).toBeNull();
	});
});
