<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { Download, Loader, RefreshCw } from '@lucide/svelte';

	let updateState = $state<UpdateState>({ status: 'idle' });
	let installing = $state(false);

	const visible = $derived(
		updateState.status === 'downloading' || updateState.status === 'ready' || installing
	);

	onMount(() => {
		const api = window.electronAPI;
		if (!api?.onUpdateState) return;

		void api.getUpdateState?.().then((current) => {
			if (current) updateState = current;
		});
		const unsubscribe = api.onUpdateState((next) => {
			updateState = next;
		});
		return () => unsubscribe?.();
	});

	async function install() {
		if (updateState.status !== 'ready' || installing) return;
		installing = true;
		try {
			await window.electronAPI?.installUpdate?.();
		} catch (error) {
			console.error('Failed to install update:', error);
			installing = false;
		}
	}
</script>

{#if visible}
	<div class="update-dock" transition:fly={{ y: 12, duration: 200 }}>
		{#if updateState.status === 'ready'}
			<button class="update-button ready" onclick={install} disabled={installing}>
				{#if installing}
					<Loader size={15} class="spin" />
					<span>Restarting…</span>
				{:else}
					<Download size={15} />
					<span
						>Restart to update{updateState.version
							? ` (v${updateState.version})`
							: ''}</span
					>
				{/if}
			</button>
		{:else if updateState.status === 'downloading'}
			<div class="update-button downloading" aria-live="polite">
				<RefreshCw size={15} class="spin" />
				<span
					>Downloading update{updateState.percent != null
						? ` ${updateState.percent}%`
						: '…'}</span
				>
			</div>
		{/if}
	</div>
{/if}

<style>
	.update-dock {
		position: fixed;
		bottom: 16px;
		left: 16px;
		z-index: 60;
	}

	.update-button {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--text-main, #1f2933);
		background: var(--panel-bg, #ffffff);
		border: 1px solid var(--border-soft, #e5e7eb);
		border-radius: var(--radius-pill, 999px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.28);
		cursor: default;
		white-space: nowrap;
	}

	.update-button.ready {
		cursor: pointer;
		border-color: var(--accent, #4c8dff);
		transition:
			transform var(--duration-fast, 160ms) var(--ease-smooth, ease),
			background var(--duration-fast, 160ms) var(--ease-smooth, ease);
	}

	.update-button.ready:hover:not(:disabled) {
		transform: translateY(-1px);
		background: var(--accent-soft, rgba(76, 141, 255, 0.16));
	}

	.update-button:disabled {
		opacity: 0.7;
		cursor: default;
	}

	.update-button :global(.spin) {
		animation: update-spin 1s linear infinite;
	}

	@keyframes update-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.update-button :global(.spin) {
			animation: none;
		}
	}
</style>
