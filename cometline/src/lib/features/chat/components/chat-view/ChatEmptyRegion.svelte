<script lang="ts">
	import EmptyChatState from '$lib/features/chat/components/EmptyChatState.svelte';

	let { bootMessage = '' }: { bootMessage?: string } = $props();
</script>

<div class="empty-region">
	<EmptyChatState />
	{#if bootMessage}
		<p class="boot-error">{bootMessage}</p>
	{/if}
</div>

<style>
	:global(.chat-home.hero-layout) .empty-region {
		position: static;
		inset: unset;
		padding: 0;
	}

	.empty-region {
		position: absolute;
		inset: 0 0 180px;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 48px 48px 0;
		flex-direction: column;
	}

	.boot-error {
		margin: 18px 0 0;
		max-width: 520px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--status-error);
		text-align: center;
	}

	@media (max-width: 900px) {
		.empty-region {
			inset: 0 0 160px;
			padding-inline: 28px;
		}
	}
</style>
