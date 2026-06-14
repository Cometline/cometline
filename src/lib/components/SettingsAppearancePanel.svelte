<script lang="ts">
	import type { HeroComposerAppearance } from '$lib/types';
	import {
		DEFAULT_HERO_COMPOSER_APPEARANCE,
		HERO_COMPOSER_PRESETS,
		heroComposerCssVarStyle,
		matchHeroComposerPreset
	} from '$lib/hero-composer-appearance';

	let {
		appearance = $bindable({ ...DEFAULT_HERO_COMPOSER_APPEARANCE })
	}: { appearance: HeroComposerAppearance } = $props();

	let previewStyle = $derived(heroComposerCssVarStyle(appearance));
	let activePreset = $derived(matchHeroComposerPreset(appearance));

	function applyPreset(preset: (typeof HERO_COMPOSER_PRESETS)[number]) {
		appearance = { ...preset.appearance };
	}

	function resetDefaults() {
		appearance = { ...DEFAULT_HERO_COMPOSER_APPEARANCE };
	}
</script>

<section class="appearance-panel">
	<div class="appearance-heading">
		<div>
			<h3>Hero composer glow</h3>
			<p>Customize the rising glow and border on the new-chat composer.</p>
		</div>
		<button class="secondary" type="button" onclick={resetDefaults}>Reset defaults</button>
	</div>

	<div class="appearance-grid">
		<div class="appearance-fields">
			<div class="preset-group">
				<span class="field-label">Presets</span>
				<div class="preset-row" role="group" aria-label="Hero glow presets">
					{#each HERO_COMPOSER_PRESETS as preset (preset.id)}
						<button
							type="button"
							class="preset-chip"
							class:selected={activePreset === preset.id}
							aria-pressed={activePreset === preset.id}
							onclick={() => applyPreset(preset)}
						>
							<span
								class="preset-swatch"
								style="background: linear-gradient(135deg, {preset.appearance
									.glowColor} 0%, {preset.appearance.ringColor} 100%)"
								aria-hidden="true"
							></span>
							{preset.label}
						</button>
					{/each}
					{#if activePreset === 'custom'}
						<span class="preset-custom">Custom</span>
					{/if}
				</div>
			</div>

			<label>
				<span>Glow color</span>
				<div class="color-field">
					<input type="color" bind:value={appearance.glowColor} aria-label="Glow color" />
					<input
						type="text"
						bind:value={appearance.glowColor}
						spellcheck="false"
						pattern="^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"
					/>
				</div>
			</label>

			<label>
				<span>Border color</span>
				<div class="color-field">
					<input type="color" bind:value={appearance.ringColor} aria-label="Border color" />
					<input
						type="text"
						bind:value={appearance.ringColor}
						spellcheck="false"
						pattern="^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"
					/>
				</div>
			</label>
		</div>

		<div class="appearance-preview" style={previewStyle}>
			<div class="preview-glow" aria-hidden="true"></div>
			<div class="preview-ring" aria-hidden="true"></div>
		</div>
	</div>
</section>

<style>
	.appearance-panel {
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		background: rgba(251, 251, 250, 0.72);
		padding: 16px;
	}

	.appearance-heading,
	.color-field,
	.preset-row {
		display: flex;
		align-items: center;
	}

	.appearance-heading {
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 16px;
	}

	.appearance-heading h3,
	.appearance-heading p {
		margin: 0;
	}

	.appearance-heading h3 {
		font-size: 15px;
		font-weight: 700;
	}

	.appearance-heading p {
		margin-top: 4px;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

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

	.preset-group {
		display: grid;
		gap: 8px;
	}

	.field-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.preset-row {
		flex-wrap: wrap;
		gap: 8px;
	}

	.preset-chip {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.76);
		padding: 6px 12px 6px 6px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	.preset-chip.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.preset-chip:hover {
		background: rgba(15, 23, 42, 0.04);
	}

	.preset-swatch {
		width: 22px;
		height: 22px;
		border-radius: 999px;
		border: 1px solid rgba(255, 255, 255, 0.8);
		box-shadow: inset 0 0 0 1px rgba(15, 23, 42, 0.08);
		flex-shrink: 0;
	}

	.preset-custom {
		font-size: 11px;
		font-weight: 600;
		color: var(--text-soft);
		padding: 0 4px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.color-field {
		gap: 8px;
	}

	input[type='color'] {
		width: 42px;
		height: 38px;
		padding: 2px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.76);
		cursor: pointer;
	}

	input[type='text'] {
		flex: 1;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.76);
		padding: 10px 11px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	input[type='text']:focus,
	input[type='color']:focus {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
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

	.secondary {
		border: none;
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-main);
	}

	.secondary:hover {
		background: rgba(15, 23, 42, 0.05);
	}

	@media (max-width: 780px) {
		.appearance-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
