<script lang="ts">
	import type { HeroComposerAppearance } from '#lib/types.js';
	import {
		heroComposerCssVarStyle,
		matchHeroComposerPreset
	} from '#lib/hero-composer-appearance.js';
	import {
		applyHeroPreset,
		selectCustomHeroPreset,
		withCustomHeroColor
	} from '#lib/features/settings/appearance-panel-hero.js';
	import HeroGlowPresets from './HeroGlowPresets.svelte';
	import HeroColorField from './HeroColorField.svelte';

	let {
		appearance = $bindable(),
		onReset
	}: {
		appearance: HeroComposerAppearance;
		onReset: () => void;
	} = $props();

	let previewStyle = $derived(heroComposerCssVarStyle(appearance));
	let activePreset = $derived(matchHeroComposerPreset(appearance));
	let customControlsDisabled = $derived(activePreset !== 'custom');
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<div>
			<h3>Hero composer glow</h3>
			<p>Customize the rising glow and border on the new-chat composer.</p>
		</div>
		<button class="secondary" type="button" onclick={onReset}>Reset defaults</button>
	</div>

	<div class="appearance-grid">
		<div class="appearance-fields">
			<HeroGlowPresets
				{appearance}
				{activePreset}
				onApplyPreset={(preset) => (appearance = applyHeroPreset(appearance, preset))}
				onSelectCustom={() => (appearance = selectCustomHeroPreset(appearance))}
			/>

			<HeroColorField
				label="Glow color"
				value={appearance.glowColor}
				disabled={customControlsDisabled}
				onInput={(value) =>
					(appearance = withCustomHeroColor(appearance, 'glowColor', value))}
			/>

			<HeroColorField
				label="Border color"
				value={appearance.ringColor}
				disabled={customControlsDisabled}
				onInput={(value) =>
					(appearance = withCustomHeroColor(appearance, 'ringColor', value))}
			/>
		</div>

		<div class="appearance-preview" style={previewStyle}>
			<div class="preview-glow" aria-hidden="true"></div>
			<div class="preview-ring" aria-hidden="true"></div>
		</div>
	</div>
</div>

<style>
	.appearance-grid {
		display: grid;
		grid-template-columns: minmax(0, 280px) minmax(0, 1fr);
		gap: 16px;
		align-items: center;
	}

	.appearance-fields {
		display: grid;
		gap: 12px;
	}

	.appearance-preview {
		position: relative;
		min-height: 168px;
		display: grid;
		place-items: center;
		padding: 28px 20px;
		border-radius: 16px;
		background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(248, 250, 252, 0.88));
		border: 1px solid var(--border-soft);
		overflow: hidden;
	}

	.preview-glow,
	.preview-ring {
		position: absolute;
		pointer-events: none;
		border-radius: 24px;
	}

	.preview-glow {
		inset: 36px 18% 28px;
		background:
			radial-gradient(
				ellipse 118% 92% at 50% 100%,
				var(--hero-composer-glow-strong),
				transparent 70%
			),
			radial-gradient(
				ellipse 88% 68% at 50% 0%,
				var(--hero-composer-glow-soft),
				transparent 74%
			);
		filter: blur(16px);
		box-shadow: 0 0 36px var(--hero-composer-glow-ring);
	}

	.preview-ring {
		inset: 44px 22% 36px;
		border: 1px solid var(--hero-composer-ring);
		box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.42) inset;
	}

	@media (max-width: 780px) {
		.appearance-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
