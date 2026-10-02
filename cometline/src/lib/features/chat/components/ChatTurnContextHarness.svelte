<script lang="ts">
	import type { Snippet } from 'svelte';
	import { setReactiveChatTurnContext } from '#lib/features/chat/chat-turn-context.js';
	import type { AssistantStackContext } from '#lib/features/chat/assistant-stack-props.js';

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
