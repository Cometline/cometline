import { describe, expect, it } from 'vitest';
import { HERO_COMPOSER_PRESET_ROSE, HERO_COMPOSER_PRESETS } from '$lib/hero-composer-appearance';
import type { HeroComposerAppearance } from '$lib/types';
import {
	applyHeroPreset,
	selectCustomHeroPreset,
	withCustomHeroColor
} from './appearance-panel-hero';

const custom = { glowColor: '#112233', ringColor: '#445566' };

describe('applyHeroPreset', () => {
	it('applies preset colors and keeps the saved custom preset', () => {
		const rose = HERO_COMPOSER_PRESETS.find((preset) => preset.id === 'rose')!;
		const next = applyHeroPreset({ presetId: 'custom', ...custom, customPreset: custom }, rose);
		expect(next).toEqual({ ...HERO_COMPOSER_PRESET_ROSE, customPreset: custom });
	});

	it('leaves customPreset undefined when none was saved', () => {
		const rose = HERO_COMPOSER_PRESETS.find((preset) => preset.id === 'rose')!;
		expect(applyHeroPreset({ presetId: 'blue', ...custom }, rose).customPreset).toBeUndefined();
	});
});

describe('selectCustomHeroPreset', () => {
	it('restores the saved custom preset', () => {
		const current: HeroComposerAppearance = {
			...HERO_COMPOSER_PRESET_ROSE,
			customPreset: custom
		};
		expect(selectCustomHeroPreset(current)).toEqual({
			presetId: 'custom',
			...custom,
			customPreset: custom
		});
	});

	it('seeds the custom preset from the current colors', () => {
		expect(selectCustomHeroPreset(HERO_COMPOSER_PRESET_ROSE)).toEqual({
			presetId: 'custom',
			glowColor: HERO_COMPOSER_PRESET_ROSE.glowColor,
			ringColor: HERO_COMPOSER_PRESET_ROSE.ringColor,
			customPreset: {
				glowColor: HERO_COMPOSER_PRESET_ROSE.glowColor,
				ringColor: HERO_COMPOSER_PRESET_ROSE.ringColor
			}
		});
	});
});

describe('withCustomHeroColor', () => {
	it('updates one color of the custom preset', () => {
		const next = withCustomHeroColor(
			{ presetId: 'custom', ...custom, customPreset: custom },
			'ringColor',
			'#abcdef'
		);
		expect(next).toEqual({
			presetId: 'custom',
			glowColor: custom.glowColor,
			ringColor: '#abcdef',
			customPreset: { glowColor: custom.glowColor, ringColor: '#abcdef' }
		});
	});
});
