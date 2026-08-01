import { describe, expect, it } from 'vitest';
import { nextReasoningEffort } from './reasoning-effort';

describe('nextReasoningEffort', () => {
	const supported = ['low', 'medium', 'high'];

	it('cycles from auto to the first supported option', () => {
		expect(nextReasoningEffort('', supported)).toBe('low');
	});

	it('advances through the supported list', () => {
		expect(nextReasoningEffort('low', supported)).toBe('medium');
		expect(nextReasoningEffort('medium', supported)).toBe('high');
	});

	it('wraps back to auto after the last option', () => {
		expect(nextReasoningEffort('high', supported)).toBe('');
	});

	it('resets to auto when current is unknown', () => {
		expect(nextReasoningEffort('xhigh', supported)).toBe('');
	});

	it('keeps the current effort when the model supports none', () => {
		expect(nextReasoningEffort('medium', [])).toBe('medium');
		expect(nextReasoningEffort('', [])).toBe('');
	});
});
