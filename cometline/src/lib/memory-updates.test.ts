import { describe, expect, it } from 'vitest';
import { memoryUpdateHint, memoryUpdateTooltip } from './memory-updates';

describe('memoryUpdateHint', () => {
	it('labels a single create as saved', () => {
		expect(
			memoryUpdateHint([
				{ action: 'create', kind: 'preference', content: 'Use Traditional Chinese' }
			])
		).toBe('Memory saved');
	});

	it('labels updates as updated', () => {
		expect(
			memoryUpdateHint([{ action: 'update', kind: 'fact', content: 'Prefers dark mode' }])
		).toBe('Memory updated');
	});
});

describe('memoryUpdateTooltip', () => {
	it('summarizes each change on its own line', () => {
		expect(
			memoryUpdateTooltip([
				{ action: 'create', kind: 'preference', content: 'Use Traditional Chinese' },
				{ action: 'update', kind: 'fact', content: 'Works on Cometline' }
			])
		).toBe('Saved preference: Use Traditional Chinese\nUpdated fact: Works on Cometline');
	});
});
