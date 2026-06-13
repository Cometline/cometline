<script lang="ts">
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { RotateCcw } from '@lucide/svelte';

	function restart() {
		window.electronAPI?.restartCometMind?.();
	}
</script>

{#if connectionState.status !== 'ready'}
	<div class="overlay">
		<div class="overlay-card">
			{#if connectionState.status === 'connecting'}
				<div class="spinner"></div>
				<p>Starting CometMind…</p>
			{:else if connectionState.status === 'error'}
				<p class="error-title">CometMind is not responding</p>
				<p class="error-message">{connectionState.message}</p>
				<p class="error-hint">
					Check <code>~/.cometmind/cometline.log</code> and make sure port 7700 is free.
				</p>
				<button class="retry-button" onclick={restart}>
					<RotateCcw size={14} />
					<span>Retry</span>
				</button>
			{/if}
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: absolute;
		inset: 0;
		background: rgba(251, 251, 250, 0.86);
		backdrop-filter: blur(4px);
		display: grid;
		place-items: center;
		z-index: 100;
	}

	.overlay-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		padding: 28px 34px;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		font-size: 14px;
		color: var(--text-muted);
	}

	.error-title {
		font-weight: 600;
		color: var(--text-main);
		margin: 0;
	}

	.error-message {
		margin: 0;
		max-width: 320px;
		text-align: center;
	}

	.error-hint {
		margin: -2px 0 2px;
		max-width: 360px;
		text-align: center;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-soft);
	}

	.error-hint code {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 11px;
		color: var(--text-muted);
	}

	.retry-button {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 7px 14px;
		border: 1px solid var(--border-soft);
		background: var(--app-bg);
		border-radius: 8px;
		font-size: 13px;
		font-weight: 500;
		color: var(--text-main);
	}

	.retry-button:hover {
		background: rgba(0, 0, 0, 0.04);
	}

	.spinner {
		width: 18px;
		height: 18px;
		border: 2px solid var(--border-soft);
		border-top-color: var(--text-muted);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
