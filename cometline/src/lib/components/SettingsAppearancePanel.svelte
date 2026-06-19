<script lang="ts">
	import type { CaretTrailSettings, HeroComposerAppearance } from '$lib/types';
	import {
		DEFAULT_HERO_COMPOSER_APPEARANCE,
		HERO_COMPOSER_PRESETS,
		heroComposerCssVarStyle,
		matchHeroComposerPreset
	} from '$lib/hero-composer-appearance';

	let {
		appearance = $bindable({ ...DEFAULT_HERO_COMPOSER_APPEARANCE }),
		caretTrail = $bindable({ enabled: true, intensity: 0.72, speed: 0.68 })
	}: { appearance: HeroComposerAppearance; caretTrail: CaretTrailSettings } = $props();

	let previewStyle = $derived(heroComposerCssVarStyle(appearance));
	let activePreset = $derived(matchHeroComposerPreset(appearance));

	function applyPreset(preset: (typeof HERO_COMPOSER_PRESETS)[number]) {
		appearance = { ...preset.appearance };
	}

	function resetDefaults() {
		appearance = { ...DEFAULT_HERO_COMPOSER_APPEARANCE };
		caretTrail = { enabled: true, intensity: 0.72, speed: 0.68 };
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

	<div class="caret-panel">
		<div class="caret-heading">
			<div>
				<h3>Input caret trail</h3>
				<p>The custom caret color follows the Hero glow color above.</p>
			</div>
			<button
				class="switch"
				class:on={caretTrail.enabled}
				role="switch"
				aria-checked={caretTrail.enabled}
				aria-label="Toggle input caret trail"
				type="button"
				onclick={() => (caretTrail = { ...caretTrail, enabled: !caretTrail.enabled })}
			>
				<span></span>
			</button>
		</div>

		<div class="slider-grid">
			<label>
				<span>Trail intensity</span>
				<input
					type="range"
					min="0"
					max="1"
					step="0.01"
					value={caretTrail.intensity}
					oninput={(e) =>
						(caretTrail = { ...caretTrail, intensity: Number(e.currentTarget.value) })}
				/>
			</label>

			<label>
				<span>Animation speed</span>
				<input
					type="range"
					min="0"
					max="1"
					step="0.01"
					value={caretTrail.speed}
					oninput={(e) =>
						(caretTrail = { ...caretTrail, speed: Number(e.currentTarget.value) })}
				/>
			</label>
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
	.caret-heading,
	.color-field,
	.preset-row {
		display: flex;
		align-items: center;
	}

	.appearance-heading,
	.caret-heading {
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 16px;
	}

	.appearance-heading h3,
	.appearance-heading p,
	.caret-heading h3,
	.caret-heading p {
		margin: 0;
	}

	.appearance-heading h3,
	.caret-heading h3 {
		font-size: 15px;
		font-weight: 700;
	}

	.appearance-heading p,
	.caret-heading p {
		margin-top: 4px;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.caret-panel {
		margin-top: 14px;
		padding-top: 14px;
		border-top: 1px solid var(--border-soft);
	}

	.slider-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 14px;
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

	input[type='range'] {
		accent-color: var(--hero-composer-glow-color);
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

	.switch {
		width: 42px;
		height: 24px;
		border: none;
		border-radius: 999px;
		padding: 3px;
		background: rgba(15, 23, 42, 0.14);
		transition: background 0.16s ease;
	}

	.switch span {
		display: block;
		width: 18px;
		height: 18px;
		border-radius: 999px;
		background: #fff;
		box-shadow: 0 2px 5px rgba(15, 23, 42, 0.18);
		transition: transform 0.16s ease;
	}

	.switch.on {
		background: var(--hero-composer-glow-color);
	}

	.switch.on span {
		transform: translateX(18px);
	}

	@media (max-width: 780px) {
		.appearance-grid {
			grid-template-columns: 1fr;
		}

		.slider-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
