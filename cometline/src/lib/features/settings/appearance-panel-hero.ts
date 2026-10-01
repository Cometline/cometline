import type { HeroComposerAppearance } from '$lib/types';
import {
	normalizeHeroComposerAppearance,
	type HeroComposerPreset
} from '$lib/hero-composer-appearance';

export type HeroColorKey = 'glowColor' | 'ringColor';

export function applyHeroPreset(
	appearance: HeroComposerAppearance,
	preset: HeroComposerPreset
): HeroComposerAppearance {
	return {
		...preset.appearance,
		customPreset: appearance.customPreset ? { ...appearance.customPreset } : undefined
	};
}

export function selectCustomHeroPreset(appearance: HeroComposerAppearance): HeroComposerAppearance {
	const normalized = normalizeHeroComposerAppearance(appearance);
	const customPreset = normalized.customPreset ?? {
		glowColor: normalized.glowColor,
		ringColor: normalized.ringColor
	};
	return {
		presetId: 'custom',
		...customPreset,
		customPreset: { ...customPreset }
	};
}

export function withCustomHeroColor(
	appearance: HeroComposerAppearance,
	key: HeroColorKey,
	value: string
): HeroComposerAppearance {
	const base = appearance.customPreset ?? {
		glowColor: appearance.glowColor,
		ringColor: appearance.ringColor
	};
	const customPreset = { ...base, [key]: value };
	return {
		presetId: 'custom',
		...customPreset,
		customPreset
	};
}
