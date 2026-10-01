import { describe, expect, it } from 'vitest';
import {
	createStreamingFoldState,
	nextStreamingFoldOverride,
	resetStreamingFoldState
} from './thread-fold';

describe('nextStreamingFoldOverride', () => {
	it('auto-expands once when streaming starts', () => {
		const state = createStreamingFoldState();
		expect(nextStreamingFoldOverride(state, 'turn-1', true)).toEqual({
			turnId: 'turn-1',
			expanded: true
		});
		expect(nextStreamingFoldOverride(state, 'turn-1', true)).toBeNull();
	});

	it('auto-collapses once when streaming ends', () => {
		const state = createStreamingFoldState();
		nextStreamingFoldOverride(state, 'turn-1', true);
		expect(nextStreamingFoldOverride(state, null, false)).toEqual({
			turnId: 'turn-1',
			expanded: false
		});
		expect(nextStreamingFoldOverride(state, null, false)).toBeNull();
	});

	it('does not collapse when streaming ends without a remembered turn', () => {
		const state = createStreamingFoldState();
		expect(nextStreamingFoldOverride(state, null, false)).toBeNull();
	});

	it('tracks a new turn after the previous one finished', () => {
		const state = createStreamingFoldState();
		nextStreamingFoldOverride(state, 'turn-1', true);
		nextStreamingFoldOverride(state, null, false);
		expect(nextStreamingFoldOverride(state, 'turn-2', true)).toEqual({
			turnId: 'turn-2',
			expanded: true
		});
	});
});

describe('resetStreamingFoldState', () => {
	it('returns empty bookkeeping', () => {
		const state = resetStreamingFoldState();
		expect(state.autoExpandedTurns.size).toBe(0);
		expect(state.autoCollapsedTurns.size).toBe(0);
		expect(state.lastStreamingTurnId).toBeNull();
	});
});
