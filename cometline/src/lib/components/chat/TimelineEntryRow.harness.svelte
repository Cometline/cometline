<script lang="ts">
	import { mockChatTurnContext } from '$lib/conversation/chat-turn-context.test-helper';
	import { setChatTurnContext } from '$lib/conversation/chat-turn-context';
	import TimelineEntryRow from './TimelineEntryRow.svelte';
	import type { TimelineEntry } from '$lib/conversation/thinking-attribution';
	import type { ChatItem } from '$lib/stores/chat.svelte';

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
