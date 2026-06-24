<script lang="ts">
	let {
		message,
		onDismiss
	}: {
		message: string;
		onDismiss?: () => void;
	} = $props();
</script>

<div class="error-banner" role="alert">
	<p>{message}</p>
	{#if onDismiss}
		<button type="button" class="dismiss" aria-label="Dismiss error" onclick={onDismiss}>
			Dismiss
		</button>
	{/if}
</div>

<style>
	.error-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 14px;
		margin: 0 var(--chat-gutter) 8px;
		border: 1px solid var(--status-error-border);
		border-radius: 12px;
		background: var(--status-error-bg);
		color: var(--status-error);
		font-size: 13px;
		line-height: 1.45;
	}

	p {
		margin: 0;
		flex: 1;
		min-width: 0;
	}

	.dismiss {
		flex-shrink: 0;
		padding: 4px 10px;
		border: 1px solid var(--status-error-border);
		border-radius: 8px;
		background: transparent;
		color: var(--status-error);
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}
</style>
