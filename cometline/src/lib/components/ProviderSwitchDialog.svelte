<script lang="ts">
	import { fly, fade } from 'svelte/transition';
	import type { ProviderSwitchWarning } from '$lib/provider-switch';

	let {
		providerName,
		warnings,
		onCancel,
		onConfirm
	}: {
		providerName: string;
		warnings: ProviderSwitchWarning[];
		onCancel: () => void;
		onConfirm: () => void;
	} = $props();
</script>

<div class="switch-scrim" transition:fade={{ duration: 100 }}>
	<button class="switch-scrim-button" aria-label="Cancel provider switch" onclick={onCancel}
	></button>
	<div
		class="switch-dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby="switch-title"
		transition:fly={{ y: 8, duration: 140 }}
	>
		<div class="switch-copy">
			<strong id="switch-title">Switch to {providerName}?</strong>
			<span>This provider handles some existing history differently:</span>
		</div>
		<ul class="switch-warnings">
			{#each warnings as warning (warning.kind)}
				<li>{warning.message}</li>
			{/each}
		</ul>
		<div class="switch-actions">
			<button class="cancel-switch" onclick={onCancel}>Cancel</button>
			<button class="confirm-switch" onclick={onConfirm}>Switch anyway</button>
		</div>
	</div>
</div>

<style>
	.switch-scrim {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: grid;
		place-items: center;
		padding: 24px;
	}

	.switch-scrim-button {
		position: absolute;
		inset: 0;
		border: none;
		background: rgba(15, 23, 42, 0.18);
		cursor: pointer;
	}

	.switch-dialog {
		position: relative;
		z-index: 1;
		width: min(420px, 100%);
		display: grid;
		gap: 12px;
		padding: 16px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.98);
		box-shadow: var(--shadow-card);
	}

	.switch-copy {
		display: grid;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.switch-copy strong {
		font-size: 14px;
		color: var(--text-main);
	}

	.switch-warnings {
		margin: 0;
		padding-left: 18px;
		display: grid;
		gap: 6px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.switch-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.cancel-switch,
	.confirm-switch {
		border: none;
		border-radius: 8px;
		padding: 7px 12px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}

	.cancel-switch {
		background: rgba(15, 23, 42, 0.05);
		color: var(--text-main);
	}

	.confirm-switch {
		background: var(--text-main);
		color: white;
	}
</style>
