import type { HeroComposerAppearance } from '$lib/types';

export interface HeroComposerPreset {
	id: 'blue' | 'rose';
	label: string;
	appearance: HeroComposerAppearance;
}

export const HERO_COMPOSER_PRESET_ROSE: HeroComposerAppearance = {
	glowColor: '#f43f5e',
	ringColor: '#fb7185'
};

/** Soft Arc-style blue — default hero glow. */
export const HERO_COMPOSER_PRESET_BLUE: HeroComposerAppearance = {
	glowColor: '#72c0ff',
	ringColor: '#4a9de8'
};

export const HERO_COMPOSER_PRESETS: HeroComposerPreset[] = [
	{ id: 'blue', label: 'Blue', appearance: HERO_COMPOSER_PRESET_BLUE },
	{ id: 'rose', label: 'Rose', appearance: HERO_COMPOSER_PRESET_ROSE }
];

export const DEFAULT_HERO_COMPOSER_APPEARANCE: HeroComposerAppearance = {
	...HERO_COMPOSER_PRESET_BLUE
};

const HEX_COLOR = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;

export function normalizeHexColor(value: string | undefined, fallback: string): string {
	if (!value) return fallback;
	const trimmed = value.trim();
	if (!HEX_COLOR.test(trimmed)) return fallback;
	if (trimmed.length === 4) {
		const [, r, g, b] = trimmed;
		return `#${r}${r}${g}${g}${b}${b}`.toLowerCase();
	}
	return trimmed.toLowerCase();
}

export function normalizeHeroComposerAppearance(
	appearance: Partial<HeroComposerAppearance> | undefined
): HeroComposerAppearance {
	return {
		glowColor: normalizeHexColor(appearance?.glowColor, DEFAULT_HERO_COMPOSER_APPEARANCE.glowColor),
		ringColor: normalizeHexColor(appearance?.ringColor, DEFAULT_HERO_COMPOSER_APPEARANCE.ringColor)
	};
}

export function heroComposerAppearanceEquals(
	a: HeroComposerAppearance,
	b: HeroComposerAppearance
): boolean {
	const left = normalizeHeroComposerAppearance(a);
	const right = normalizeHeroComposerAppearance(b);
	return left.glowColor === right.glowColor && left.ringColor === right.ringColor;
}

export function matchHeroComposerPreset(
	appearance: HeroComposerAppearance
): HeroComposerPreset['id'] | 'custom' {
	const normalized = normalizeHeroComposerAppearance(appearance);
	for (const preset of HERO_COMPOSER_PRESETS) {
		if (heroComposerAppearanceEquals(normalized, preset.appearance)) return preset.id;
	}
	return 'custom';
}

export function hexToRgba(hex: string, alpha: number): string {
	const normalized = normalizeHexColor(hex, DEFAULT_HERO_COMPOSER_APPEARANCE.glowColor);
	const r = Number.parseInt(normalized.slice(1, 3), 16);
	const g = Number.parseInt(normalized.slice(3, 5), 16);
	const b = Number.parseInt(normalized.slice(5, 7), 16);
	return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

export function heroComposerCssVars(
	appearance: HeroComposerAppearance
): Record<string, string> {
	const colors = normalizeHeroComposerAppearance(appearance);
	return {
		'--hero-composer-glow-color': colors.glowColor,
		'--hero-composer-glow-strong': hexToRgba(colors.glowColor, 0.52),
		'--hero-composer-glow-soft': hexToRgba(colors.glowColor, 0.24),
		'--hero-composer-glow-ring': hexToRgba(colors.glowColor, 0.14),
		'--hero-composer-ring': hexToRgba(colors.ringColor, 0.24),
		'--pane-focus-border': hexToRgba(colors.ringColor, 0.42),
		'--pane-focus-glow': hexToRgba(colors.glowColor, 0.06),
		'--user-bubble-bg': `color-mix(in srgb, ${colors.glowColor} 70%, #1f2933)`,
		'--user-bubble-shadow': hexToRgba(colors.glowColor, 0.22)
	};
}

export function heroComposerCssVarStyle(appearance: HeroComposerAppearance): string {
	return Object.entries(heroComposerCssVars(appearance))
		.map(([key, value]) => `${key}: ${value}`)
		.join('; ');
}
