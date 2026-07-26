// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from 'vitest';
import { findSessionTextMatches } from './session-find';

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
