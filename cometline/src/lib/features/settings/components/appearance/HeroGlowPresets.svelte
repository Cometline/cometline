<script lang="ts">
	import type { HeroComposerAppearance } from '$lib/types';
	import {
		HERO_COMPOSER_PRESETS,
		type HeroComposerPreset,
		type HeroComposerPresetSelection
	} from '$lib/hero-composer-appearance';

	let {
		appearance,
		activePreset,
		onApplyPreset,
		onSelectCustom
	}: {
		appearance: HeroComposerAppearance;
		activePreset: HeroComposerPresetSelection;
		onApplyPreset: (preset: HeroComposerPreset) => void;
		onSelectCustom: () => void;
	} = $props();

	let hasCustomPreset = $derived(Boolean(appearance.customPreset));
</script>

<div class="preset-group">
	<span class="field-label">Presets</span>
	<div class="preset-row" role="group" aria-label="Hero glow presets">
		{#each HERO_COMPOSER_PRESETS as preset (preset.id)}
			<button
				type="button"
				class="preset-chip"
				class:selected={activePreset === preset.id}
				aria-pressed={activePreset === preset.id}
				onclick={() => onApplyPreset(preset)}
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
		<button
			type="button"
			class="preset-chip custom-preset-chip"
			class:selected={activePreset === 'custom'}
			aria-pressed={activePreset === 'custom'}
			onclick={onSelectCustom}
		>
			<span
				class="preset-swatch"
				class:empty={!hasCustomPreset}
				style={hasCustomPreset
					? `background: linear-gradient(135deg, ${appearance.customPreset?.glowColor} 0%, ${appearance.customPreset?.ringColor} 100%)`
					: 'background: linear-gradient(135deg, var(--color-23232a) 0%, var(--color-454553) 100%)'}
				aria-hidden="true"
			></span>
			Custom
		</button>
	</div>
</div>

<style>
	.preset-row {
		display: flex;
		align-items: center;
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
		background: rgba(15, 23, 42, 0.08);
	}

	.custom-preset-chip {
		padding-right: 12px;
	}

	.preset-swatch {
		width: 22px;
		height: 22px;
		border-radius: 999px;
		border: 1px solid rgba(255, 255, 255, 0.8);
		box-shadow: inset 0 0 0 1px rgba(15, 23, 42, 0.08);
		flex-shrink: 0;
	}

	.preset-swatch.empty {
		background:
			linear-gradient(
				90deg,
				transparent 9px,
				rgba(15, 23, 42, 0.16) 9px 11px,
				transparent 11px
			),
			linear-gradient(
				0deg,
				transparent 9px,
				rgba(15, 23, 42, 0.16) 9px 11px,
				transparent 11px
			),
			rgba(15, 23, 42, 0.04);
	}
</style>
