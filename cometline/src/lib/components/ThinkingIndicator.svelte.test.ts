import { describe, expect, it } from 'vitest';
import source from './ThinkingIndicator.svelte?raw';
import { THINKING_INDICATOR_VARIANTS } from './thinking-indicator';

function orbitRotations() {
	const orbit = /@keyframes thinking-orbit\s*\{([\s\S]*?)\n\t\}/.exec(source)?.[1] ?? '';
	return [...orbit.matchAll(/rotate\((-?\d+)deg\)/g)].map((match) => Number(match[1]));
}

describe('ThinkingIndicator', () => {
	it('keeps the comet tail rotation continuous around the orbit', () => {
		const rotations = orbitRotations();

		expect(rotations).toEqual([90, 32, 0, -32, -90, -148, -180, -212, -270]);
		for (let index = 1; index < rotations.length; index += 1) {
			expect(rotations[index]).toBeLessThan(rotations[index - 1]);
		}
	});

	it('exposes the celestial variant set', () => {
		expect([...THINKING_INDICATOR_VARIANTS]).toEqual(['orbit', 'eclipse', 'nova']);
		for (const variant of THINKING_INDICATOR_VARIANTS) {
			expect(source).toContain(`variant-${variant}`);
		}
	});
});
