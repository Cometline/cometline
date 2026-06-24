<script lang="ts">
	import { fade } from 'svelte/transition';
	import { Brain } from '@lucide/svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import EventCard from '$lib/components/chat/EventCard.svelte';
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
		{#if item.memories.length > 0}
			{#key memoryCycleTick}
				{@const mem = item.memories[memoryCycleTick % item.memories.length]}
				<div class="memory-chip-rotator">
					<span
						class="memory-chip memory-chip-cycling"
						in:fade={{ duration: 500 }}
						title={mem.content}>{mem.kind}: {mem.content}</span
					>
				</div>
			{/key}
		{/if}
	</EventCard>
</ThreadRow>

<style>
	.memory-chip {
		display: block;
		width: 100%;
		min-width: 0;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		padding: 5px 10px;
		border-radius: 10px;
		background: rgba(0, 102, 204, 0.08);
		color: var(--text-main);
		font-size: 11px;
		line-height: 1.45;
	}

	.memory-chip-rotator {
		display: grid;
		min-width: 0;
		width: 100%;
	}

	.memory-chip-cycling {
		grid-column: 1;
		grid-row: 1;
	}
</style>
