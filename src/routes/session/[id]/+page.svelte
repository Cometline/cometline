<script lang="ts">
	import { page } from '$app/state';
	import ChatView from '$lib/components/ChatView.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';

	let sessionId = $derived(page.params.id);
	let initialMessage = $state<string | null>(null);
	let ready = $state(false);

	$effect(() => {
		if (!sessionId || ready) return;
		const session = sessionStore.sessions.find((item) => item.id === sessionId);
		if (session) {
			sessionStore.selectSession(session);
		}
		initialMessage = sessionStore.takePendingMessage(sessionId);
		ready = true;
	});
</script>

{#if ready && sessionId}
	<ChatView sessionId={sessionId} bootMessage={shellStore.bootMessage} {initialMessage} />
{/if}
