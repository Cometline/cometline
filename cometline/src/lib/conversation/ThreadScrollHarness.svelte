<script lang="ts">
	import { createThreadScroll } from './thread-scroll.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	let {
		sessionId = 'session-1',
		items = [],
		streaming = false,
		synced = true,
		loading = false,
		cached = true
	}: {
		sessionId?: string;
		items?: ChatItem[];
		streaming?: boolean;
		synced?: boolean;
		loading?: boolean;
		cached?: boolean;
	} = $props();

	let scrollerEl = $state<HTMLDivElement>();
	let lastUserId = $derived(items.findLast((item) => item.type === 'user')?.id ?? null);
	let userMessageCount = $derived(
		items.reduce((count, item) => (item.type === 'user' ? count + 1 : count), 0)
	);

	const scroll = createThreadScroll({
		getSessionId: () => sessionId,
		getIsSessionSynced: () => synced,
		getThreadItems: () => items,
		getSessionStreaming: () => streaming,
		getLastUserId: () => lastUserId,
		getUserMessageCount: () => userMessageCount,
		getIsLoading: () => loading,
		sessionHasCachedTranscript: () => cached
	});

	$effect(() => {
		scroll.setScroller(scrollerEl);
	});
</script>

<div
	bind:this={scrollerEl}
	data-testid="thread-scroll"
	data-active-min-height={scroll.activeTurnMinHeight}
	data-active-pinned-user-id={scroll.activePinnedUserId ?? ''}
	data-initial-paint={scroll.isInitialTranscriptPaint}
	data-last-user-id={lastUserId}
	data-streaming={streaming}
	data-user-count={userMessageCount}
	data-viewport-height={scroll.viewportHeight}
>
	{#each items as item (item.id)}
		{#if item.type === 'user'}
			<div data-user-item-id={item.id}>{item.text}</div>
		{/if}
	{/each}
</div>
