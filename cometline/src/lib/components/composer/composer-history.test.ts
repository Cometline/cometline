import { describe, expect, it } from 'vitest';
import {
	buildRecallList,
	coerceHistoryEntry,
	listUserMessageTexts,
	parseHistoryJsonl,
	serializeHistoryEntry,
	stepHistoryIndex,
	trimHistoryEntries,
	type ComposerHistoryEntry
} from './composer-history';

function entry(
	display: string,
	workspacePath = '/repo',
	sessionId = 's1',
	timestamp = 1
): ComposerHistoryEntry {
	return { display, workspacePath, sessionId, timestamp };
}

describe('parseHistoryJsonl / serializeHistoryEntry', () => {
	it('round-trips entries and skips corrupt lines', () => {
		const a = entry('hello', '/repo', 's1', 10);
		const raw = [serializeHistoryEntry(a), '{bad', '', serializeHistoryEntry(entry('world'))].join(
			'\n'
		);
		const parsed = parseHistoryJsonl(raw);
		expect(parsed).toHaveLength(2);
		expect(parsed[0]?.display).toBe('hello');
		expect(parsed[1]?.display).toBe('world');
	});

	it('accepts Claude-style project field via coerceHistoryEntry', () => {
		expect(
			coerceHistoryEntry({
				display: 'hi',
				project: '/old',
				sessionId: 'x',
				timestamp: 1
			})
		).toEqual({
			display: 'hi',
			workspacePath: '/old',
			sessionId: 'x',
			timestamp: 1
		});
	});
});

describe('trimHistoryEntries', () => {
	it('keeps the newest max entries', () => {
		const entries = [entry('a'), entry('b'), entry('c'), entry('d')];
		expect(trimHistoryEntries(entries, 2).map((e) => e.display)).toEqual(['c', 'd']);
	});
});

describe('buildRecallList', () => {
	it('puts pending first, then newest workspace history, then transcript gaps', () => {
		const list = buildRecallList({
			pendingText: 'unsent draft',
			workspacePath: '/repo',
			historyEntries: [
				entry('older', '/repo', 's1', 1),
				entry('newer', '/repo', 's2', 2),
				entry('other-ws', '/other', 's3', 3)
			],
			transcriptUserTexts: ['newer', 'only-in-transcript']
		});
		expect(list).toEqual(['unsent draft', 'newer', 'older', 'only-in-transcript']);
	});

	it('dedupes pending against history and normalizes trailing slashes on workspace', () => {
		const list = buildRecallList({
			pendingText: 'same',
			workspacePath: '/repo/',
			historyEntries: [entry('same', '/repo', 's1', 1), entry('prev', '/repo', 's1', 2)]
		});
		expect(list).toEqual(['same', 'prev']);
	});
});

describe('listUserMessageTexts', () => {
	it('returns newest-first user texts', () => {
		expect(
			listUserMessageTexts([
				{ type: 'user', text: 'one' },
				{ type: 'assistant', text: 'ok' },
				{ type: 'user', text: 'two' },
				{ type: 'user', text: '  ' }
			])
		).toEqual(['two', 'one']);
	});
});

describe('stepHistoryIndex', () => {
	it('enters at 0 on up from live draft', () => {
		expect(stepHistoryIndex(null, 'up', 3)).toEqual({ index: 0 });
	});

	it('walks up/down and exits to live draft below 0', () => {
		expect(stepHistoryIndex(0, 'up', 3)).toEqual({ index: 1 });
		expect(stepHistoryIndex(2, 'up', 3)).toEqual({ index: 2 });
		expect(stepHistoryIndex(1, 'down', 3)).toEqual({ index: 0 });
		expect(stepHistoryIndex(0, 'down', 3)).toEqual({ index: null });
		expect(stepHistoryIndex(null, 'down', 3)).toEqual({ index: null });
		expect(stepHistoryIndex(null, 'up', 0)).toEqual({ index: null });
	});
});
