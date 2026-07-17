import { describe, expect, it } from 'vitest';
import { assistantThinkingWaitStatus } from './assistant-wait-status';

describe('assistantThinkingWaitStatus', () => {
	it('frames turn phases as thinking with a detail line', () => {
		expect(assistantThinkingWaitStatus('compacting_context', undefined, 0)).toEqual({
			label: 'Thinking',
			detail: 'Summarizing earlier context…'
		});
	});

	it('uses custom activity messages as detail', () => {
		expect(assistantThinkingWaitStatus(undefined, 'Preparing tools…', 0)).toEqual({
			label: 'Thinking',
			detail: 'Preparing tools…'
		});
	});

	it('escalates elapsed wait copy without implying no response', () => {
		expect(assistantThinkingWaitStatus(undefined, undefined, 10)).toEqual({
			label: 'Still thinking',
			detail: '10s'
		});
		expect(assistantThinkingWaitStatus(undefined, undefined, 45).detail).toContain(
			'slower response'
		);
		expect(assistantThinkingWaitStatus(undefined, undefined, 120).detail).toContain(
			'Long responses can take a few minutes'
		);
		expect(assistantThinkingWaitStatus(undefined, undefined, 120).detail).not.toContain(
			'time out'
		);
	});

	it('defaults to a neutral thinking state', () => {
		expect(assistantThinkingWaitStatus(undefined, undefined, 0)).toEqual({
			label: 'Thinking',
			detail: 'Thinking…'
		});
	});
});
