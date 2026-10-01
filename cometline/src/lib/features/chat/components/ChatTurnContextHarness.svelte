<script lang="ts">
	import type { Snippet } from 'svelte';
	import { setReactiveChatTurnContext } from '$lib/features/chat/chat-turn-context';
	import type { AssistantStackContext } from '$lib/features/chat/assistant-stack-props';

	let {
		ctx,
		children
	}: {
		ctx: AssistantStackContext;
		children: Snippet;
	} = $props();

	setReactiveChatTurnContext(() => ctx);
</script>

{@render children()}
