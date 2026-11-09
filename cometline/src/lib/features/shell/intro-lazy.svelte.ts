import { shellStore } from '#lib/stores/shell.svelte.js';

type IntroGateComponent =
	typeof import('#lib/features/shell/components/IntroAnimationGate.svelte').default;

/** Lazy-load the intro gate once; used by the root overlay host and AppShell preload. */
export function createIntroLazyLoader() {
	let Intro = $state<IntroGateComponent | null>(null);
	let loadPromise: Promise<IntroGateComponent | null> | null = null;

	function loadIntroAnimation() {
		if (Intro) return Promise.resolve(Intro);
		if (!loadPromise) {
			loadPromise = import('#lib/features/shell/components/IntroAnimationGate.svelte')
				.then((module) => {
					Intro = module.default;
					return module.default;
				})
				.catch((error) => {
					loadPromise = null;
					console.error('Intro animation failed to load', error);
					shellStore.closeIntro();
					return null;
				});
		}
		return loadPromise;
	}

	return {
		get Intro() {
			return Intro;
		},
		loadIntroAnimation
	};
}
