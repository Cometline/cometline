import { describe, expect, it } from 'vitest';
import type { StreamEvent, TranscriptItem } from '#lib/types.js';
import { initChatState, reduceChatState } from '#lib/reducers/chat.js';
import { itemsFromTranscript } from '#lib/stores/chat-transcript.js';
import type { StreamCtx } from '#lib/stores/chat-stream-types.js';

/** Mirrors applyEventToSession: reducer state minus needsTextSeparator is lost between events. */
function reduceStreamEventsBroken(events: StreamEvent[]) {
	const ctx: StreamCtx = {
		assistant: { current: null },
		reasoning: { current: null },
		needsTextSeparator: false
	};
	let items: ReturnType<typeof initChatState>['items'] = [];
	let nextId = 0;
	for (const event of events) {
		const reduced = reduceChatState(
			{
				items,
				error: '',
				assistant: ctx.assistant.current,
				reasoning: ctx.reasoning.current,
				nextId,
				contextBudget: null
			},
			event
		);
		items = reduced.items;
		nextId = reduced.nextId;
		ctx.assistant.current = reduced.assistant;
		ctx.reasoning.current = reduced.reasoning;
	}
	const assistant = items.find((item) => item.type === 'assistant');
	return assistant?.type === 'assistant' ? assistant.text : '';
}

function reduceStreamEventsLive(events: StreamEvent[]) {
	const ctx: StreamCtx = {
		assistant: { current: null },
		reasoning: { current: null },
		needsTextSeparator: false
	};
	let items: ReturnType<typeof initChatState>['items'] = [];
	let nextId = 0;
	for (const event of events) {
		const reduced = reduceChatState(
			{
				items,
				error: '',
				assistant: ctx.assistant.current,
				reasoning: ctx.reasoning.current,
				needsTextSeparator: ctx.needsTextSeparator,
				nextId,
				contextBudget: null
			},
			event
		);
		items = reduced.items;
		nextId = reduced.nextId;
		ctx.assistant.current = reduced.assistant;
		ctx.reasoning.current = reduced.reasoning;
		ctx.needsTextSeparator = reduced.needsTextSeparator ?? false;
	}
	const assistant = items.find((item) => item.type === 'assistant');
	return assistant?.type === 'assistant' ? assistant.text : '';
}

function assistantTextFromTranscript(transcript: TranscriptItem[]) {
	const items = itemsFromTranscript(transcript);
	const assistant = items.find((item) => item.type === 'assistant');
	return assistant?.type === 'assistant' ? assistant.text : '';
}

describe('assistant text parity (live stream vs reload)', () => {
	const toolBeatTranscript: TranscriptItem[] = [
		{ type: 'assistant', text: 'Let me check.\n' },
		{ type: 'tool', tool_name: 'list_dir', tool_input: { path: '.' } },
		{ type: 'assistant', text: 'Found it.' }
	];

	const toolBeatEvents: StreamEvent[] = [
		{ type: 'text_delta', delta: 'Let me check.\n' },
		{ type: 'tool_call', id: 'tc-1', tool: 'list_dir', input: { path: '.' } },
		{ type: 'tool_result', id: 'tc-1', tool: 'list_dir', output: 'src\nREADME.md' },
		{ type: 'text_delta', delta: 'Found it.' },
		{ type: 'done' }
	];

	it('documents that live stream dropped beat separators before ctx carried the flag', () => {
		const reloaded = assistantTextFromTranscript(toolBeatTranscript);
		expect(reloaded).toBe('Let me check.\n\nFound it.');
		expect(reduceStreamEventsBroken(toolBeatEvents)).toBe('Let me check.\nFound it.');
	});

	it('matches itemsFromTranscript for text → tool → text without reasoning', () => {
		const expected = assistantTextFromTranscript(toolBeatTranscript);
		expect(reduceStreamEventsLive(toolBeatEvents)).toBe(expected);
	});

	it('matches itemsFromTranscript when reasoning separates beats', () => {
		const transcript: TranscriptItem[] = [
			{ type: 'assistant', text: 'I found one source.' },
			{ type: 'reasoning', text: 'Find another source.' },
			{ type: 'assistant', text: 'Here is another source.' }
		];
		const events: StreamEvent[] = [
			{ type: 'text_delta', delta: 'I found one source.' },
			{ type: 'reasoning_start' },
			{ type: 'reasoning_delta', text: 'Find another source.' },
			{ type: 'text_delta', delta: 'Here is another source.' },
			{ type: 'done' }
		];
		expect(reduceStreamEventsLive(events)).toBe(assistantTextFromTranscript(transcript));
	});

	it('matches itemsFromTranscript when the next beat delta starts with whitespace', () => {
		const transcript: TranscriptItem[] = [
			{ type: 'assistant', text: 'Before tool.' },
			{ type: 'tool', tool_name: 'read_file', tool_input: { path: 'a.txt' } },
			{ type: 'assistant', text: '  After tool.' }
		];
		const events: StreamEvent[] = [
			{ type: 'text_delta', delta: 'Before tool.' },
			{ type: 'tool_call', id: 'tc-2', tool: 'read_file', input: { path: 'a.txt' } },
			{ type: 'tool_result', id: 'tc-2', tool: 'read_file', output: 'ok' },
			{ type: 'text_delta', delta: '  After tool.' },
			{ type: 'done' }
		];
		expect(reduceStreamEventsLive(events)).toBe(assistantTextFromTranscript(transcript));
	});
});
