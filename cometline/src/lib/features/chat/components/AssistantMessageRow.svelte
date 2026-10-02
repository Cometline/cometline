<script lang="ts">
	import AssistantStack from '#lib/features/chat/components/AssistantStack.svelte';
	import ThreadAvatar from '#lib/features/chat/components/ThreadAvatar.svelte';
	import ThreadRow from '#lib/features/chat/components/ThreadRow.svelte';
	import {
		assistantStackBindings,
		type AssistantStackContext
	} from '#lib/features/chat/assistant-stack-props.js';
	import { startsSpeakerRun } from '#lib/features/chat/thread-view-helpers.js';
	import type { ChatItem } from '#lib/stores/chat.svelte.js';

	type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

	let {
		item,
		threadItems,
		index,
		avatarSrc,
		avatarSrcset,
		stackContext,
		showActivitySpinner,
		hideAvatarForFirstTurn = false,
		deferMarkdown = false
	}: {
		item: AssistantItem;
		threadItems: readonly ChatItem[];
		index: number;
		avatarSrc: string;
		avatarSrcset?: string;
		stackContext: AssistantStackContext;
		showActivitySpinner: (item: AssistantItem) => boolean;
		hideAvatarForFirstTurn?: boolean;
		deferMarkdown?: boolean;
	} = $props();

	const continuationRow = $derived(!startsSpeakerRun(threadItems, index, 'assistant'));
	const startsRun = $derived(startsSpeakerRun(threadItems, index, 'assistant'));
</script>

<ThreadRow variant="assistant" {continuationRow}>
	{#if startsRun}
		<ThreadAvatar
			variant="avatar"
			{avatarSrc}
			{avatarSrcset}
			flightHidden={hideAvatarForFirstTurn}
		/>
	{:else}
		<ThreadAvatar variant="gutter" {avatarSrc} {avatarSrcset} />
	{/if}
	<div class="assistant-column" class:first-turn-destination-hidden={hideAvatarForFirstTurn}>
		<AssistantStack
			{...assistantStackBindings(
				stackContext,
				item,
				showActivitySpinner(item),
				deferMarkdown
			)}
		/>
	</div>
</ThreadRow>

<style>
	.assistant-column {
		min-width: 0;
		flex: 1 1 0;
	}

	.first-turn-destination-hidden {
		visibility: hidden;
	}
</style>
