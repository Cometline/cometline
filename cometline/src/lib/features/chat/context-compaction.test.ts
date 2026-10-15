import { describe, expect, it } from 'vitest';
import { compactionFeedback } from './context-compaction';

describe('compactionFeedback', () => {
	it('toasts a successful compaction', () => {
		expect(compactionFeedback({ status: 'compacted' })).toEqual({
			tone: 'success',
			label: 'Context compacted',
			detail: 'Earlier messages were folded into the session summary.'
		});
	});

	it('warns when there is nothing older to fold', () => {
		expect(compactionFeedback({ status: 'skipped', reason: 'no_prefix' })).toMatchObject({
			tone: 'warning',
			detail: 'Recent messages already fit beside the summary.'
		});
	});

	it('stays quiet when the user cancels', () => {
		expect(compactionFeedback({ status: 'skipped', reason: 'cancelled' })).toBeNull();
	});
});
