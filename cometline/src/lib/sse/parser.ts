import type { StreamEvent } from '$lib/types';

/** Parse a single SSE line. Returns the event, the sentinel 'done', or null. */
export function parseSSEData(line: string): StreamEvent | 'done' | null {
	const trimmed = line.trim();
	if (!trimmed.startsWith('data:')) return null;

	const payload = trimmed.slice(5).trim();
	if (!payload) return null;
	if (payload === '[DONE]') return 'done';

	try {
		return JSON.parse(payload) as StreamEvent;
	} catch {
		return null;
	}
}

/** Split accumulated SSE text into complete lines, keeping the trailing
 *  incomplete fragment as the new buffer. */
export function parseSSELines(buffer: string, text: string): { lines: string[]; buffer: string } {
	const combined = buffer + text;
	const lines = combined.split('\n');
	const remaining = lines.pop() ?? '';
	return { lines, buffer: remaining };
}

/** Feed a chunk of SSE text into the parser and emit parsed results.
 *  This is a stateful convenience over the pure `parseSSELines`/`parseSSEData`
 *  functions; it owns the line buffer so callers can stream chunks directly. */
export function createSSEParser() {
	let buffer = '';

	function feed(chunk: string): (StreamEvent | 'done' | null)[] {
		const { lines, buffer: next } = parseSSELines(buffer, chunk);
		buffer = next;
		return lines.map(parseSSEData);
	}

	function flush(): (StreamEvent | 'done' | null)[] {
		const trailing = buffer.trim();
		buffer = '';
		if (!trailing) return [];
		return [parseSSEData(trailing)];
	}

	return { feed, flush };
}
