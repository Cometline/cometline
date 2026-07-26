<script lang="ts">
	import { Brain } from '@lucide/svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import EventCard from '$lib/components/chat/EventCard.svelte';
	import MemoryCard from '$lib/components/chat/MemoryCard.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	let {
		item,
		memoryCycleTick
	}: {
		item: Extract<ChatItem, { type: 'memory' }>;
		memoryCycleTick: number;
	} = $props();
</script>

<ThreadRow variant="event">
	<EventCard variant="memory">
		<div class="event-title">
			<Brain size={14} /><span>Memories used · {item.memories.length}</span>
		</div>
		<MemoryCard
			memories={item.memories}
			expanded={true}
			contentOnly={true}
			cycling={item.memories.length > 1}
			cycleTick={memoryCycleTick}
			onToggle={() => {}}
		/>
	</EventCard>
</ThreadRow>
