<script lang="ts">
	let { failed, onRetry }: { failed: boolean; onRetry: () => void } = $props();
</script>

<aside class="workspace-panel-loading" aria-label="Loading workspace panel" aria-busy="true">
	{#if failed}
		<button type="button" onclick={onRetry}>Retry workspace panel</button>
	{:else}
		<span>Loading workspace…</span>
	{/if}
</aside>

<style>
	.workspace-panel-loading {
		display: grid;
		flex: 0 1 auto;
		width: var(--workspace-panel-slot-width);
		max-width: 100%;
		min-width: 0;
		height: 100%;
		place-items: center;
		box-sizing: border-box;
		border-left: 1px solid var(--border-soft);
		background: var(--panel-bg);
		color: var(--text-muted);
		font-size: 13px;
	}

	.workspace-panel-loading button {
		border: 0;
		background: transparent;
		color: var(--accent);
		font: inherit;
		cursor: pointer;
	}
</style>
