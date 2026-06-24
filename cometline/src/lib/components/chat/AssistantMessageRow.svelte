<script lang="ts">
	import AssistantStack from '$lib/components/chat/AssistantStack.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import {
		assistantStackBindings,
		type AssistantStackContext
	} from '$lib/conversation/assistant-stack-props';
	import { startsSpeakerRun } from '$lib/conversation/thread-view-helpers';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import type { IconVariant } from '$lib/types';

	type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

	let {
		item,
		threadItems,
		index,
		iconVariant,
		stackContext,
		showActivitySpinner,
		hideAvatarForFirstTurn = false
	}: {
		item: AssistantItem;
		threadItems: readonly ChatItem[];
		index: number;
		iconVariant: IconVariant;
		stackContext: AssistantStackContext;
		showActivitySpinner: (item: AssistantItem) => boolean;
		hideAvatarForFirstTurn?: boolean;
	} = $props();

	const continuationRow = $derived(!startsSpeakerRun(threadItems, index, 'assistant'));
	const startsRun = $derived(startsSpeakerRun(threadItems, index, 'assistant'));
</script>

<ThreadRow variant="assistant" {continuationRow}>
	{#if startsRun}
		<ThreadAvatar variant="avatar" {iconVariant} flightHidden={hideAvatarForFirstTurn} />
	{:else}
		<ThreadAvatar variant="gutter" {iconVariant} />
	{/if}
	<div class="assistant-column" class:first-turn-destination-hidden={hideAvatarForFirstTurn}>
		<AssistantStack
			{...assistantStackBindings(stackContext, item, showActivitySpinner(item))}
		/>
	</div>
</ThreadRow>

<style>
	.assistant-column {
		min-width: 0;
		flex: 1;
		max-width: var(--chat-assistant-column);
	}

	.first-turn-destination-hidden {
		visibility: hidden;
	}
</style>
