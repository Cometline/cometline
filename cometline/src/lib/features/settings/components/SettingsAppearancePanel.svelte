<script lang="ts">
	import type {
		CaretTrailSettings,
		HeroComposerAppearance,
		ResponseCompleteSoundSettings,
		TerminalAppearanceSettings
	} from '#lib/types.js';
	import { DEFAULT_HERO_COMPOSER_APPEARANCE } from '#lib/hero-composer-appearance.js';
	import { DEFAULT_TERMINAL_APPEARANCE } from '#lib/features/workspace/terminal-appearance.js';
	import { defaultResponseCompleteSoundSettings } from '#lib/features/settings/schema.js';
	import HeroGlowSection from './appearance/HeroGlowSection.svelte';
	import TerminalAppearanceSection from './appearance/TerminalAppearanceSection.svelte';
	import CaretTrailSection from './appearance/CaretTrailSection.svelte';
	import ResponseSoundSection from './appearance/ResponseSoundSection.svelte';

	let {
		appearance = $bindable({ ...DEFAULT_HERO_COMPOSER_APPEARANCE }),
		caretTrail = $bindable({ enabled: true, intensity: 0.72, speed: 0.68 }),
		terminal = $bindable({ ...DEFAULT_TERMINAL_APPEARANCE }),
		responseCompleteSound = $bindable(defaultResponseCompleteSoundSettings())
	}: {
		appearance: HeroComposerAppearance;
		caretTrail: CaretTrailSettings;
		terminal: TerminalAppearanceSettings;
		responseCompleteSound: ResponseCompleteSoundSettings;
	} = $props();

	function resetDefaults() {
		appearance = { ...DEFAULT_HERO_COMPOSER_APPEARANCE };
		caretTrail = { enabled: true, intensity: 0.72, speed: 0.68 };
		terminal = { ...DEFAULT_TERMINAL_APPEARANCE };
		responseCompleteSound = defaultResponseCompleteSoundSettings();
	}
</script>

<section class="appearance-panel settings-panel-frame">
	<div class="settings-panel-body">
		<HeroGlowSection bind:appearance onReset={resetDefaults} />
		<TerminalAppearanceSection bind:terminal />
		<CaretTrailSection bind:caretTrail />
		<ResponseSoundSection bind:responseCompleteSound />
	</div>
</section>
