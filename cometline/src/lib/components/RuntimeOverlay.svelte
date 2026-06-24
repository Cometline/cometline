<script lang="ts">
	import { connectionState } from '$lib/stores/runtime.svelte';
	import ThinkingIndicator from '$lib/components/ThinkingIndicator.svelte';
	import { RefreshCw, TriangleAlert } from '@lucide/svelte';

	function retry() {
		connectionState.reconnect();
	}
</script>

{#if connectionState.status === 'connecting'}
	<div class="overlay" role="status" aria-live="polite" aria-label="Starting CometMind">
		<div class="overlay-card">
			<ThinkingIndicator size={24} label="Starting CometMind" />
			<p>Starting CometMind…</p>
		</div>
	</div>
{:else if connectionState.status === 'error'}
	<div class="overlay" role="alert" aria-live="assertive">
		<div class="overlay-card error-card">
			<div class="error-heading">
				<TriangleAlert size={20} aria-hidden="true" />
				<h2>Cannot reach CometMind</h2>
			</div>
			<p class="error-message">{connectionState.message}</p>
			<p class="error-hint">
				Check <code>~/.cometmind/cometline.log</code> for sidecar errors, then retry.
			</p>
			<button type="button" class="retry-button" onclick={retry}>
				<RefreshCw size={14} aria-hidden="true" />
				Retry connection
			</button>
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: absolute;
		inset: 0;
		background: var(--overlay-scrim);
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
		max-width: min(420px, calc(100% - 2rem));
	}

	.error-card {
		align-items: stretch;
		text-align: left;
	}

	.error-heading {
		display: flex;
		align-items: center;
		gap: 10px;
		color: var(--status-error);
	}

	.error-heading h2 {
		margin: 0;
		font-size: 16px;
		font-weight: 650;
		color: var(--text-main);
	}

	.error-message {
		margin: 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--status-error);
	}

	.error-hint {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.error-hint code {
		font-size: 11px;
		background: rgba(15, 23, 42, 0.05);
		padding: 1px 5px;
		border-radius: 4px;
	}

	.retry-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		margin-top: 4px;
		padding: 10px 16px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg);
		color: var(--text-main);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.retry-button:hover {
		background: rgba(255, 255, 255, 0.92);
		border-color: color-mix(in srgb, var(--status-error) 24%, var(--border-soft));
	}
</style>
