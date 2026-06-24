<script lang="ts">
	import { TriangleAlert } from '@lucide/svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import EventCard from '$lib/components/chat/EventCard.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	let { item }: { item: Extract<ChatItem, { type: 'error' }> } = $props();
</script>

<ThreadRow variant="event">
	<EventCard variant="error">
		<div class="event-title">
			<TriangleAlert size={14} /><span>Error</span>
		</div>
		<p class="error-text">{item.text}</p>
	</EventCard>
</ThreadRow>

<style>
	.error-text {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
	}
</style>
