<script lang="ts">
	import { mockChatTurnContext } from '#lib/features/chat/chat-turn-context.test-helper.js';
	import { setChatTurnContext } from '#lib/features/chat/chat-turn-context.js';
	import TimelineEntryRow from './TimelineEntryRow.svelte';
	import type { TimelineEntry } from '#lib/features/chat/thinking-attribution.js';
	import type { ChatItem } from '#lib/stores/chat.svelte.js';

	let { entry }: { entry: TimelineEntry } = $props();

	const baseCtx = mockChatTurnContext();
	setChatTurnContext({
		...baseCtx,
		fold: {
			...baseCtx.fold,
			thinkingExpanded: () => true
		}
	});

	const assistant: Extract<ChatItem, { type: 'assistant' }> = {
		id: 'asst-1',
		type: 'assistant',
		text: 'Done',
		pending: false
	};
</script>

<TimelineEntryRow {entry} {assistant} assistantId="asst-1" />
