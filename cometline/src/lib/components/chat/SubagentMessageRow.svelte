<script lang="ts">
	import SubagentPanel from '$lib/components/chat/SubagentPanel.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import { startsSpeakerRun } from '$lib/conversation/thread-view-helpers';
	import type { AssistantStackFoldController } from '$lib/conversation/assistant-stack-props';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	let {
		item,
		threadItems,
		index,
		avatarSrc,
		avatarSrcset,
		fold
	}: {
		item: Extract<ChatItem, { type: 'subagent' }>;
		threadItems: readonly ChatItem[];
		index: number;
		avatarSrc: string;
		avatarSrcset?: string;
		fold: AssistantStackFoldController;
	} = $props();
</script>

<ThreadRow
	variant="assistant"
	class="subagent-row"
	continuationRow={!startsSpeakerRun(threadItems, index, 'assistant')}
>
	<ThreadAvatar variant="gutter" {avatarSrc} {avatarSrcset} />
	<div class="subagent-stack">
		<SubagentPanel
			{item}
			expanded={fold.subagentExpanded(item.id)}
			onToggle={() => fold.toggleSubagent(item.id)}
		/>
	</div>
</ThreadRow>

<style>
	:global(.subagent-row.continuation-row) {
		margin-top: -16px;
	}

	.subagent-stack {
		min-width: 0;
		flex: 1;
		max-width: var(--chat-assistant-column);
	}
</style>
