<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		variant = 'default',
		class: className = '',
		children
	}: {
		variant?: 'memory' | 'error' | 'default';
		class?: string;
		children: Snippet;
	} = $props();
</script>

<div class="event-card {variant}-card {className}">
	{@render children()}
</div>

<style>
	.event-card {
		min-width: 0;
		width: 100%;
		max-width: var(--chat-content-column);
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.68);
		border-radius: 14px;
		padding: 10px 12px;
		color: var(--text-muted);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.04);
	}

	.error-card {
		color: var(--status-error);
		background: var(--status-error-bg);
		border-color: var(--status-error-border);
	}

	:global(.event-title) {
		display: flex;
		align-items: center;
		gap: 7px;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
		margin-bottom: 7px;
	}

	:global(.event-title svg:last-child) {
		flex-shrink: 0;
	}
</style>
