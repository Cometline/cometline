import { describe, expect, it } from 'vitest';
import { shouldApplyComposerFocus } from './composer-focus';

const base = {
	requestId: 3,
	requestSessionId: 'sess-1',
	sessionId: 'sess-1',
	focusedPane: 'chat' as const,
	lastAppliedRequestId: 2
};

describe('shouldApplyComposerFocus', () => {
	it('applies a new request for the open session on the chat pane', () => {
		expect(shouldApplyComposerFocus(base)).toBe(true);
	});

	it('ignores a leftover request while the terminal is focused', () => {
		expect(shouldApplyComposerFocus({ ...base, focusedPane: 'terminal' })).toBe(false);
	});

	it('does not refocus when only the composer instance identity changed', () => {
		expect(shouldApplyComposerFocus({ ...base, lastAppliedRequestId: 3 })).toBe(false);
	});

	it('ignores a request aimed at another session', () => {
		expect(shouldApplyComposerFocus({ ...base, requestSessionId: 'sess-2' })).toBe(false);
	});
});
