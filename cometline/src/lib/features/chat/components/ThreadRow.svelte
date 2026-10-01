<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		variant = 'default',
		continuationRow = false,
		class: className = '',
		children,
		...rest
	}: {
		variant?: 'user' | 'assistant' | 'event' | 'default';
		continuationRow?: boolean;
		class?: string;
		children: Snippet;
		[key: string]: unknown;
	} = $props();
</script>

<div
	class="thread-row {variant}-row {className}"
	class:continuation-row={continuationRow}
	{...rest}
>
	{@render children()}
</div>

<style>
	.thread-row {
		display: flex;
		width: 100%;
		gap: var(--chat-row-gap);
	}

	.continuation-row {
		margin-top: -6px;
	}

	.user-row {
		justify-content: flex-start;
		align-items: flex-start;
	}

	.assistant-row {
		justify-content: flex-start;
		align-items: flex-start;
	}

	.event-row {
		justify-content: flex-start;
	}
</style>
