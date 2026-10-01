<script lang="ts">
	import type { Snippet } from 'svelte';
	let { children }: { children: Snippet } = $props();
</script>

<div class="slash-menu-details">
	{@render children()}
</div>

<style>
	.slash-menu-details :global(.mention-option) {
		flex-direction: row;
		align-items: center;
		gap: 8px;
		padding: 7px 10px;
	}

	.slash-menu-details :global(.mention-option svg) {
		flex-shrink: 0;
		color: var(--text-soft);
	}

	.slash-menu-details :global(.mention-path) {
		font-size: 12px;
		font-weight: 500;
		color: var(--text-main);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.slash-menu-details :global(.mention-hint) {
		margin: 2px 0 0;
		padding: 6px 10px 2px;
		font-size: 10px;
		color: var(--text-soft);
		border-top: 1px solid var(--border-soft);
	}

	.slash-menu-details :global(.model-command-option) {
		position: relative;
		padding-right: 28px;
	}

	.slash-menu-details :global(.model-command-option.is-selected) {
		background: rgba(0, 102, 204, 0.04);
	}

	.slash-menu-details :global(.model-command-name) {
		display: inline-flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 4px;
		min-width: 0;
	}

	.slash-menu-details :global(.model-command-label) {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		min-width: 0;
	}

	.slash-menu-details :global(.model-command-context-hint) {
		font-weight: 500;
		color: var(--text-soft);
	}

	.slash-menu-details :global(.model-command-check) {
		position: absolute;
		right: 10px;
		top: 50%;
		transform: translateY(-50%);
		color: rgba(0, 102, 204, 0.7);
	}
</style>
