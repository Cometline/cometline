<script lang="ts">
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { SettingsMemoryPanel } from '#lib/features/settings/settings-memory-panel.svelte.js';

	let { panel }: { panel: SettingsMemoryPanel } = $props();
</script>

{#if panel.s.settings}
	<div class="settings-section">
		<div class="settings-section-heading">
			<div>
				<h3>Retrieval & lifecycle</h3>
				<p>Control when memories are retrieved, extracted, and aged out.</p>
			</div>
		</div>

		<div class="settings-grid">
			<div class="memory-composition-note">
				<strong>What gets added to a prompt</strong>
				<p>
					Up to 3 user preferences, {panel.s.settings.task_outcome_limit} relevant task
					{panel.s.settings.task_outcome_limit === 1 ? 'outcome' : 'outcomes'}, and
					{panel.s.settings.max_retrieved} semantic
					{panel.s.settings.max_retrieved === 1 ? 'memory' : 'memories'}. These groups
					share 5% of the available context, capped at 4,096 tokens.
				</p>
			</div>

			<div class="toggles">
				<SettingsToggle
					label="Auto retrieve"
					bind:checked={panel.s.settings.auto_retrieve}
				/>
				<SettingsToggle
					label="Auto summarize"
					bind:checked={panel.s.settings.auto_extract}
				/>
			</div>

			<div class="sliders">
				<label>
					<span
						>Similarity threshold ({Math.round(
							panel.s.settings.similarity_threshold * 100
						)}%)</span
					>
					<input
						type="range"
						min="0"
						max="1"
						step="0.05"
						bind:value={panel.s.settings.similarity_threshold}
					/>
				</label>
				<label>
					<span>Semantic memories in prompt ({panel.s.settings.max_retrieved})</span>
					<input
						type="range"
						min="1"
						max="20"
						bind:value={panel.s.settings.max_retrieved}
					/>
				</label>
				<label>
					<span>Task outcomes in prompt ({panel.s.settings.task_outcome_limit})</span>
					<input
						type="range"
						min="1"
						max="10"
						bind:value={panel.s.settings.task_outcome_limit}
					/>
					<p class="field-hint">
						Semantically relevant task outcomes, up to {panel.s.settings
							.task_outcome_limit}.
					</p>
				</label>
				<label>
					<span
						>Decay half-life (days): {panel.s.settings.lifecycle
							.decay_half_life_days}</span
					>
					<input
						type="range"
						min="7"
						max="90"
						bind:value={panel.s.settings.lifecycle.decay_half_life_days}
					/>
				</label>
				<label>
					<span>Max memories: {panel.s.settings.lifecycle.max_memories}</span>
					<input
						type="range"
						min="100"
						max="2000"
						step="50"
						bind:value={panel.s.settings.lifecycle.max_memories}
					/>
				</label>
			</div>
		</div>
	</div>
{/if}

<style>
	.settings-grid {
		display: grid;
		gap: 14px;
	}

	.memory-composition-note {
		display: grid;
		gap: 3px;
		padding: 10px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(0, 102, 204, 0.04);
		font-size: 12px;
		line-height: 1.45;
	}

	.memory-composition-note strong {
		color: var(--text-main);
	}

	.memory-composition-note p {
		margin: 0;
		color: var(--text-muted);
	}

	.toggles {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
	}

	.sliders {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	input[type='range'] {
		width: 100%;
	}

	@media (max-width: 780px) {
		.toggles,
		.sliders {
			grid-template-columns: 1fr;
		}
	}
</style>
