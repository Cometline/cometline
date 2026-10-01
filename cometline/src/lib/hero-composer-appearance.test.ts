import { describe, expect, it } from 'vitest';
import {
	DEFAULT_HERO_COMPOSER_APPEARANCE,
	HERO_COMPOSER_PRESETS,
	HERO_COMPOSER_PRESET_CLAY,
	normalizeHeroComposerAppearance
} from './hero-composer-appearance';

describe('hero composer presets', () => {
	it('defaults to the clay glow and gray border', () => {
		expect(HERO_COMPOSER_PRESETS.map((preset) => preset.id)).toEqual(['clay', 'blue', 'rose']);
		expect(DEFAULT_HERO_COMPOSER_APPEARANCE).toEqual(HERO_COMPOSER_PRESET_CLAY);
		expect(normalizeHeroComposerAppearance(undefined)).toEqual(HERO_COMPOSER_PRESET_CLAY);
	});

	it('keeps an explicit blue or rose selection', () => {
		expect(normalizeHeroComposerAppearance({ presetId: 'blue' }).presetId).toBe('blue');
		expect(normalizeHeroComposerAppearance({ presetId: 'rose' }).presetId).toBe('rose');
	});
});
