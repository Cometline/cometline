import { describe, expect, it } from 'vitest';
import { createSSEParser, parseSSEData, parseSSELines } from './parser';
import type { StreamEvent } from '$lib/types';

describe('parseSSEData', () => {
	it('ignores empty lines and comments', () => {
		expect(parseSSEData('')).toBeNull();
		expect(parseSSEData(': keep-alive')).toBeNull();
		expect(parseSSEData('event: message')).toBeNull();
	});

	it('returns done sentinel for [DONE]', () => {
		expect(parseSSEData('data: [DONE]')).toBe('done');
	});

	it('parses a JSON event payload', () => {
		const event: StreamEvent = { type: 'text_delta', delta: 'hello' };
		expect(parseSSEData(`data: ${JSON.stringify(event)}`)).toEqual(event);
	});

	it('returns null for malformed JSON', () => {
		expect(parseSSEData('data: {not json}')).toBeNull();
	});
});

describe('parseSSELines', () => {
	it('buffers incomplete trailing lines', () => {
		const first = parseSSELines('', 'data: {"type":"text_');
		expect(first.lines).toEqual([]);
		expect(first.buffer).toBe('data: {"type":"text_');

		const second = parseSSELines(first.buffer, 'delta","delta":"hi"}\ndata: [DONE]');
		expect(second.lines).toHaveLength(1);
		expect(second.lines[0]).toBe('data: {"type":"text_delta","delta":"hi"}');
		expect(second.buffer).toBe('data: [DONE]');
	});

	it('handles multiple newlines in one chunk', () => {
		const ev1: StreamEvent = { type: 'text_delta', delta: 'a' };
		const ev2: StreamEvent = { type: 'done' };
		const { lines, buffer } = parseSSELines(
			'',
			`data: ${JSON.stringify(ev1)}\n\ndata: ${JSON.stringify(ev2)}\n`
		);
		expect(lines).toEqual([`data: ${JSON.stringify(ev1)}`, '', `data: ${JSON.stringify(ev2)}`]);
		expect(buffer).toBe('');
	});
});

describe('createSSEParser', () => {
	it('streams events across chunk boundaries', () => {
		const parser = createSSEParser();
		const ev: StreamEvent = { type: 'text_delta', delta: 'world' };

		expect(parser.feed('data: {"type":"text_')).toEqual([]);
		expect(parser.feed('delta","delta":"world"}\n')).toEqual([ev]);
		expect(parser.feed('data: [DONE]\n')).toEqual(['done']);
	});

	it('flushes the trailing line', () => {
		const parser = createSSEParser();
		const ev: StreamEvent = { type: 'done' };
		parser.feed(`data: ${JSON.stringify(ev)}`);
		expect(parser.flush()).toEqual([ev]);
	});
});
