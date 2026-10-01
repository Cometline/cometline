<script lang="ts">
	import type { ResponseCompleteSoundSettings } from '$lib/types';
	import { playResponseCompleteSound } from '$lib/sound/response-complete';

	let {
		responseCompleteSound = $bindable()
	}: { responseCompleteSound: ResponseCompleteSoundSettings } = $props();

	let volumePercent = $derived(Math.round(responseCompleteSound.volume * 100));

	function previewResponseSound() {
		playResponseCompleteSound({
			...responseCompleteSound,
			force: true
		});
	}
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<div>
			<h3>Agent run sound</h3>
			<p>Play a short chime when an agent run completes, is stopped, or fails.</p>
		</div>
		<button
			class="switch"
			class:on={responseCompleteSound.enabled}
			role="switch"
			aria-checked={responseCompleteSound.enabled}
			aria-label="Toggle agent run sound"
			type="button"
			onclick={() =>
				(responseCompleteSound = {
					...responseCompleteSound,
					enabled: !responseCompleteSound.enabled
				})}
		>
			<span></span>
		</button>
	</div>

	<div class="sound-settings-row">
		<label class="sound-volume-field">
			<span class="sound-volume-label">
				<span>Volume</span>
				<span class="sound-volume-value" aria-live="polite">{volumePercent}%</span>
			</span>
			<input
				type="range"
				min="0"
				max="1"
				step="0.01"
				value={responseCompleteSound.volume}
				disabled={!responseCompleteSound.enabled}
				aria-label="Agent run sound volume"
				aria-valuetext={`${volumePercent}%`}
				oninput={(e) =>
					(responseCompleteSound = {
						...responseCompleteSound,
						volume: Number(e.currentTarget.value)
					})}
			/>
		</label>
		<button
			class="secondary preview-sound-button"
			type="button"
			disabled={responseCompleteSound.volume <= 0}
			onclick={previewResponseSound}
		>
			Preview
		</button>
	</div>
</div>

<style>
	.sound-settings-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 14px;
		align-items: end;
	}

	.sound-volume-field {
		min-width: 0;
	}

	.sound-volume-label {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 12px;
	}

	.sound-volume-value {
		font-variant-numeric: tabular-nums;
		color: var(--text-main);
	}

	.preview-sound-button {
		height: 38px;
		padding: 0 14px;
		white-space: nowrap;
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

	input:disabled {
		cursor: not-allowed;
		opacity: 0.56;
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
		.sound-settings-row {
			grid-template-columns: 1fr;
		}

		.preview-sound-button {
			width: 100%;
		}
	}
</style>
