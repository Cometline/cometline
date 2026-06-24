<script lang="ts">
	import { mockChatTurnContext } from '$lib/conversation/chat-turn-context.test-helper';
	import { setChatTurnContext } from '$lib/conversation/chat-turn-context';
	import AssistantActivityGroup from './AssistantActivityGroup.svelte';
	import type { TimelineEntry } from '$lib/conversation/thinking-attribution';
	import type { ChatItem } from '$lib/stores/chat.svelte';

	let {
		timeline,
		parentExpanded = false,
		onToggleParent = () => {}
	}: {
		timeline: TimelineEntry[];
		parentExpanded?: boolean;
		onToggleParent?: () => void;
	} = $props();

	setChatTurnContext(mockChatTurnContext());

	const assistant: Extract<ChatItem, { type: 'assistant' }> = {
		id: 'asst-1',
		type: 'assistant',
		text: 'Hello',
		pending: false
	};
</script>

<AssistantActivityGroup
	{assistant}
	assistantId="asst-1"
	{timeline}
	{parentExpanded}
	{onToggleParent}
	timelineEntryKey={(entry) => `${entry.kind}-0`}
	showThinkingSpinner={false}
/>
