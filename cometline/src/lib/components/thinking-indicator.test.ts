import { describe, expect, it } from 'vitest';
import {
	stageForTurnPhase,
	variantForStage,
	variantForTurnPhase
} from './thinking-indicator';
import { StickyThinkingVariant } from './sticky-thinking-variant';

describe('thinking indicator stage map', () => {
	it('merges wire phases into three UI stages', () => {
		expect(stageForTurnPhase('retrieving_memories')).toBe('thinking');
		expect(stageForTurnPhase('compacting_context')).toBe('thinking');
		expect(stageForTurnPhase('contacting_model')).toBe('thinking');
		expect(stageForTurnPhase('continuing')).toBe('thinking');
		expect(stageForTurnPhase('running_tools')).toBe('tools');
		expect(stageForTurnPhase('composing_response')).toBe('composing');
		expect(stageForTurnPhase(undefined)).toBe('thinking');
		expect(stageForTurnPhase('unknown_future_phase')).toBe('thinking');
	});

	it('maps stages to orbit / eclipse / nova', () => {
		expect(variantForStage('thinking')).toBe('orbit');
		expect(variantForStage('tools')).toBe('eclipse');
		expect(variantForStage('composing')).toBe('nova');
		expect(variantForTurnPhase('running_tools')).toBe('eclipse');
		expect(variantForTurnPhase('retrieving_memories')).toBe('orbit');
	});
});

describe('StickyThinkingVariant', () => {
	it('snaps on first observe then holds until stable', () => {
		let now = 0;
		const sticky = new StickyThinkingVariant(750, () => now);

		expect(sticky.observe('contacting_model')).toEqual({
			variant: 'orbit',
			stage: 'thinking',
			commitInMs: 0
		});

		expect(sticky.observe('running_tools')).toEqual({
			variant: 'orbit',
			stage: 'thinking',
			commitInMs: 750
		});

		now = 400;
		expect(sticky.observe('running_tools').variant).toBe('orbit');
		expect(sticky.observe('running_tools').commitInMs).toBe(350);

		now = 750;
		expect(sticky.observe('running_tools')).toEqual({
			variant: 'eclipse',
			stage: 'tools',
			commitInMs: 0
		});
	});

	it('ignores wire-phase chatter inside the same UI stage', () => {
		let now = 0;
		const sticky = new StickyThinkingVariant(750, () => now);
		sticky.observe('retrieving_memories');
		now = 100;
		expect(sticky.observe('compacting_context')).toEqual({
			variant: 'orbit',
			stage: 'thinking',
			commitInMs: 0
		});
		expect(sticky.observe('contacting_model')).toEqual({
			variant: 'orbit',
			stage: 'thinking',
			commitInMs: 0
		});
	});

	it('resets the hold when the pending stage changes', () => {
		let now = 0;
		const sticky = new StickyThinkingVariant(750, () => now);
		sticky.observe('contacting_model');

		now = 100;
		expect(sticky.observe('running_tools').commitInMs).toBe(750);

		now = 500;
		expect(sticky.observe('composing_response').commitInMs).toBe(750);
		expect(sticky.observe('composing_response').variant).toBe('orbit');

		now = 1249;
		expect(sticky.observe('composing_response').variant).toBe('orbit');

		now = 1250;
		expect(sticky.observe('composing_response')).toEqual({
			variant: 'nova',
			stage: 'composing',
			commitInMs: 0
		});
	});

	it('cancels a pending change when stage returns to displayed', () => {
		let now = 0;
		const sticky = new StickyThinkingVariant(750, () => now);
		sticky.observe('contacting_model');
		now = 10;
		sticky.observe('running_tools');
		now = 100;
		expect(sticky.observe('contacting_model')).toEqual({
			variant: 'orbit',
			stage: 'thinking',
			commitInMs: 0
		});
	});
});
