<script lang="ts">
	import type { CaretTrailSettings } from '$lib/types';

	let { caretTrail = $bindable() }: { caretTrail: CaretTrailSettings } = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
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
					(caretTrail = {
						...caretTrail,
						intensity: Number(e.currentTarget.value)
					})}
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

<style>
	.slider-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 14px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	input[type='range'] {
		accent-color: var(--hero-composer-glow-color);
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
		background: var(--panel-bg);
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
		.slider-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
