// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from 'vitest';
import type { ChatItem } from '#lib/stores/chat.svelte.js';
import { groupThreadItemsIntoTurns } from './thread-turns';
import { findSessionItemMatches, findSessionTextMatches } from './session-find';

describe('findSessionTextMatches', () => {
	let transcript: HTMLDivElement;

	beforeEach(() => {
		transcript = document.createElement('div');
		document.body.replaceChildren(transcript);
	});

	it('matches case-insensitively across inline markdown nodes', () => {
		transcript.innerHTML = `
			<div data-session-find-text><p>Hello <strong>formatted</strong> world</p></div>
		`;
		const matches = findSessionTextMatches(transcript, 'HELLO formatted world');
		expect(matches).toHaveLength(1);
		expect(matches[0]?.range.toString()).toBe('Hello formatted world');
	});

	it('normalizes rendered whitespace and treats regex characters literally', () => {
		transcript.innerHTML = `
			<div data-session-find-text><p>Use   value.* <em>here</em></p></div>
		`;
		expect(findSessionTextMatches(transcript, 'value.* here')).toHaveLength(1);
	});

	it('does not match across separate messages or ignored controls', () => {
		transcript.innerHTML = `
			<div data-session-find-text>first <button>hidden button text</button></div>
			<div data-session-find-text>second</div>
		`;
		expect(findSessionTextMatches(transcript, 'first second')).toHaveLength(0);
		expect(findSessionTextMatches(transcript, 'hidden button text')).toHaveLength(0);
	});

	it('returns matches in transcript order', () => {
		transcript.innerHTML = `
			<div data-session-find-text>match one match</div>
			<div data-session-find-text>another match</div>
		`;
		const matches = findSessionTextMatches(transcript, 'match');
		expect(matches.map((match) => match.range.toString())).toEqual(['match', 'match', 'match']);
	});
});

describe('findSessionItemMatches', () => {
	it('searches user and assistant plain text with turn indices', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'alpha needle' },
			{ id: 'a1', type: 'assistant', text: 'nope' },
			{ id: 'u2', type: 'user', text: 'beta' },
			{ id: 'a2', type: 'assistant', text: 'needle here and needle again' }
		];
		const turns = groupThreadItemsIntoTurns(items);
		const matches = findSessionItemMatches(turns, 'NEEDLE');
		expect(matches).toEqual([
			{ itemId: 'u1', turnId: 'u1', turnIndex: 0, occurrenceInItem: 0 },
			{ itemId: 'a2', turnId: 'u2', turnIndex: 1, occurrenceInItem: 0 },
			{ itemId: 'a2', turnId: 'u2', turnIndex: 1, occurrenceInItem: 1 }
		]);
	});

	it('ignores tool/status rows and empty queries', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'hi' },
			{ id: 't1', type: 'tool', toolName: 'read', input: {}, output: 'needle in tool' },
			{ id: 'a1', type: 'assistant', text: 'ok' }
		];
		const turns = groupThreadItemsIntoTurns(items);
		expect(findSessionItemMatches(turns, 'needle')).toEqual([]);
		expect(findSessionItemMatches(turns, '   ')).toEqual([]);
	});
});
