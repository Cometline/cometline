<script lang="ts">
	import type { Snippet } from 'svelte';
	let { children }: { children: Snippet } = $props();
</script>

<div class="slash-menu-details">
	{@render children()}
</div>

<style>
	.slash-menu-details {
		display: flex;
		flex-direction: column;
		align-items: stretch;
		justify-content: flex-start;
	}

	/* Include .skill-command-option so this beats the column rule in SlashCommandMenu.
	   Equal specificity loses across component stylesheets and stacks the icon above the path. */
	.slash-menu-details :global(.skill-command-option.mention-option) {
		flex: 0 0 auto;
		flex-direction: row;
		align-items: center;
		justify-content: flex-start;
		gap: 8px;
		padding: 7px 10px;
		text-align: left;
	}

	.slash-menu-details :global(.skill-command-option.mention-option svg) {
		flex-shrink: 0;
		color: var(--text-soft);
	}

	.slash-menu-details :global(.mention-path) {
		flex: 1;
		min-width: 0;
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

	.slash-menu-details :global(.skill-command-option.model-command-option) {
		position: relative;
		padding-right: 28px;
	}

	.slash-menu-details :global(.skill-command-option.model-command-option.is-selected) {
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
