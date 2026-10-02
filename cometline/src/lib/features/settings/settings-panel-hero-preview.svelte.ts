import { heroComposerCssVars } from '#lib/hero-composer-appearance.js';
import { settingsStore } from '#lib/stores/settings.svelte.js';
import type { HeroComposerAppearance } from '#lib/types.js';

/** Must be called during component initialisation; restores saved vars on teardown. */
export function previewDraftHeroComposer(getHeroComposer: () => HeroComposerAppearance) {
	$effect(() => {
		const vars = heroComposerCssVars(getHeroComposer());
		const root = document.documentElement;
		for (const [key, value] of Object.entries(vars)) {
			root.style.setProperty(key, value);
		}
		return () => {
			const saved = heroComposerCssVars(settingsStore.settings.appearance.heroComposer);
			for (const [key, value] of Object.entries(saved)) {
				root.style.setProperty(key, value);
			}
		};
	});
}
