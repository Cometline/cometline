<script lang="ts">
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import ThreadRow from '$lib/components/chat/ThreadRow.svelte';
	import { startsSpeakerRun } from '$lib/conversation/thread-view-helpers';
	import type { AssistantStackFoldController } from '$lib/conversation/assistant-stack-props';
	import type { ChatItem } from '$lib/stores/chat.svelte';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';
	import type { JobResource } from '$lib/client/cometmind';

	let {
		item,
		threadItems,
		index,
		avatarSrc,
		avatarSrcset,
		sessionId,
		toolFoldLabel,
		fold,
		onNotifyAgent,
		onStartJob
	}: {
		item: Extract<ChatItem, { type: 'tool' }>;
		threadItems: readonly ChatItem[];
		index: number;
		avatarSrc: string;
		avatarSrcset?: string;
		sessionId: string;
		toolFoldLabel: (tool: Extract<ChatItem, { type: 'tool' }>) => string;
		fold: AssistantStackFoldController;
		onNotifyAgent?: (payload: ChatTurnPayload) => void | Promise<void>;
		onStartJob?: (job: JobResource) => void | Promise<void>;
	} = $props();
</script>

<ThreadRow
	variant="assistant"
	class="tool-row"
	continuationRow={!startsSpeakerRun(threadItems, index, 'assistant')}
>
	<ThreadAvatar variant="gutter" {avatarSrc} {avatarSrcset} />
	<div class="tool-stack">
		<ToolFoldPanel
			{item}
			label={toolFoldLabel(item)}
			expanded={fold.toolOutputExpanded(item)}
			onToggle={() => fold.toggleToolOutput(item.id)}
			{sessionId}
			{onNotifyAgent}
			{onStartJob}
		/>
	</div>
</ThreadRow>

<style>
	:global(.tool-row.continuation-row) {
		margin-top: -16px;
	}

	.tool-stack {
		min-width: 0;
		flex: 1;
		max-width: var(--chat-assistant-column);
	}
</style>
