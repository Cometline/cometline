<script lang="ts">
	import EventCard from '$lib/components/chat/EventCard.svelte';
	import { TriangleAlert } from '@lucide/svelte';

	let { variant = 'default' as 'default' | 'error' } = $props();
</script>

<EventCard {variant}>
	<div class="event-title">
		{#if variant === 'error'}
			<TriangleAlert size={14} /><span>Error</span>
		{:else}
			<span>Memory saved</span>
		{/if}
	</div>
	<p style="margin:0;font-size:12px;">
		{variant === 'error' ? 'Stream interrupted.' : 'User prefers concise answers.'}
	</p>
</EventCard>
