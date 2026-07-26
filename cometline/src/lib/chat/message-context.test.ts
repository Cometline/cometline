// @vitest-environment jsdom

import { describe, expect, it, vi } from 'vitest';
import {
	messageContextLabel,
	messageContextRefsFromPending,
	messageContextRefsFromWebContexts,
	openMessageContext,
	parseFileContextSource
} from './message-context';

describe('messageContextRefsFromWebContexts', () => {
	it('maps wire contexts to slim UI refs and marks viewing files', () => {
		const refs = messageContextRefsFromWebContexts([
			{
				kind: 'file',
				title: 'notes.md',
				source: 'workspace-file:notes.md',
				content: ''
			},
			{
				kind: 'file',
				title: 'notes.md:2-3',
				source: 'workspace-file:notes.md#L2-L3',
				content: 'line two'
			},
			{
				kind: 'page',
				title: 'Example',
				source: 'https://example.com',
				content: 'body'
			}
		]);
		expect(refs).toEqual([
			{
				kind: 'file',
				title: 'notes.md',
				source: 'workspace-file:notes.md',
				role: 'viewing'
			},
			{
				kind: 'file',
				title: 'notes.md:2-3',
				source: 'workspace-file:notes.md#L2-L3'
			},
			{
				kind: 'page',
				title: 'Example',
				source: 'https://example.com'
			}
		]);
	});
});

describe('messageContextRefsFromPending', () => {
	it('preserves viewing role from pending chips', () => {
		const refs = messageContextRefsFromPending([
			{
				kind: 'file',
				title: 'app.ts',
				source: 'workspace-file:src/app.ts',
				content: '',
				role: 'viewing'
			}
		]);
		expect(refs[0]?.role).toBe('viewing');
		expect(messageContextLabel(refs[0]!)).toBe('Viewing app.ts');
	});
});

describe('parseFileContextSource', () => {
	it('parses workspace file path and line range', () => {
		expect(parseFileContextSource('workspace-file:src/app.ts#L2-L5')).toEqual({
			path: 'src/app.ts',
			range: { startLine: 2, endLine: 5 }
		});
	});

	it('parses wiki paths without range', () => {
		expect(parseFileContextSource('@runtime/wiki/notes.md')).toEqual({
			path: '@runtime/wiki/notes.md',
			range: null
		});
	});

	it('parses single-line anchors', () => {
		expect(parseFileContextSource('workspace-file:foo.ts#L12')).toEqual({
			path: 'foo.ts',
			range: { startLine: 12, endLine: 12 }
		});
	});
});

describe('assistant response contexts', () => {
	it('uses the selected-text title as the chip label', () => {
		expect(
			messageContextLabel({
				kind: 'message',
				title: 'A useful selected passage',
				source: 'assistant-response://sess-1/2'
			})
		).toBe('A useful selected passage');
	});

	it('scrolls to and highlights the source response', () => {
		const target = document.createElement('div');
		target.dataset.assistantResponseSource = 'assistant-response://sess-1/2';
		const scrollIntoView = vi.fn();
		target.scrollIntoView = scrollIntoView;
		document.body.appendChild(target);

		openMessageContext({
			kind: 'message',
			title: 'Selected passage',
			source: 'assistant-response://sess-1/2'
		});

		expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' });
		expect(target).toHaveClass('response-context-highlight');
		target.remove();
	});
});
