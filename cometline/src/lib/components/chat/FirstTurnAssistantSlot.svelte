<script lang="ts">
	import { assistantThinkingWait } from '$lib/conversation/thread-format';
	import type { AssistantStackContext } from '$lib/conversation/assistant-stack-props';
	import { assistantStackBindings } from '$lib/conversation/assistant-stack-props';
	import AssistantStack from '$lib/components/chat/AssistantStack.svelte';
	import AssistantThinkingWait from '$lib/components/chat/AssistantThinkingWait.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

	let {
		avatarSrc,
		avatarSrcset,
		firstTurnHandoffPending,
		firstAssistantItem,
		sessionStreaming,
		stackContext,
		showAssistantRow,
		showActivitySpinner,
		flightPlaceholder = false,
		ariaHidden = false
	}: {
		avatarSrc: string;
		avatarSrcset?: string;
		firstTurnHandoffPending: boolean;
		firstAssistantItem: AssistantItem | undefined;
		sessionStreaming: boolean;
		stackContext: AssistantStackContext;
		showAssistantRow: (item: AssistantItem) => boolean;
		showActivitySpinner: (item: AssistantItem) => boolean;
		flightPlaceholder?: boolean;
		ariaHidden?: boolean;
	} = $props();

	const hideDestination = $derived(firstTurnHandoffPending);
</script>

<div
	class="row assistant-row"
	class:flight-placeholder={flightPlaceholder}
	aria-hidden={ariaHidden || undefined}
>
	<ThreadAvatar
		variant="avatar"
		{avatarSrc}
		{avatarSrcset}
		flightHidden={firstTurnHandoffPending}
		flightTarget="avatar"
	/>
	{#if firstAssistantItem && showAssistantRow(firstAssistantItem)}
		<div class="assistant-column" class:first-turn-destination-hidden={hideDestination}>
			<AssistantStack
				{...assistantStackBindings(
					stackContext,
					firstAssistantItem,
					showActivitySpinner(firstAssistantItem)
				)}
			/>
		</div>
	{:else if sessionStreaming}
		<div class="assistant-stack" class:first-turn-destination-hidden={hideDestination}>
			<AssistantThinkingWait
				label={assistantThinkingWait(undefined, stackContext.now).label}
				detail={assistantThinkingWait(undefined, stackContext.now).detail}
				color={stackContext.heroGlowColor}
			/>
		</div>
	{:else}
		<div class="assistant-stack" class:first-turn-destination-hidden={hideDestination}></div>
	{/if}
</div>

<style>
	.row {
		display: flex;
		width: 100%;
		gap: var(--chat-row-gap);
	}

	.assistant-row {
		justify-content: flex-start;
		align-items: flex-start;
	}

	.assistant-column {
		min-width: 0;
		flex: 1;
		max-width: var(--chat-assistant-column);
	}

	.flight-placeholder {
		pointer-events: none;
	}

	.flight-placeholder :global(.assistant-stack) {
		min-height: 1px;
	}

	.first-turn-destination-hidden {
		visibility: hidden;
	}
</style>
