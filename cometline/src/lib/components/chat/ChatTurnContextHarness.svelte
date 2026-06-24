<script lang="ts">
	import type { Snippet } from 'svelte';
	import { setChatTurnContext } from '$lib/conversation/chat-turn-context';
	import type { AssistantStackContext } from '$lib/conversation/assistant-stack-props';

	let {
		ctx,
		children
	}: {
		ctx: AssistantStackContext;
		children: Snippet;
	} = $props();

	$effect(() => {
		setChatTurnContext(ctx);
	});
</script>

{@render children()}
