<script lang="ts">
	import SubagentPanel from '$lib/features/chat/components/SubagentPanel.svelte';
	import ThreadAvatar from '$lib/features/chat/components/ThreadAvatar.svelte';
	import ThreadRow from '$lib/features/chat/components/ThreadRow.svelte';
	import { startsSpeakerRun } from '$lib/features/chat/thread-view-helpers';
	import type { AssistantStackFoldController } from '$lib/features/chat/assistant-stack-props';
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
		flex: 1 1 0;
	}
</style>
