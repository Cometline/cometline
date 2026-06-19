<script lang="ts">
	import { page } from '$app/state';
	import ChatView from '$lib/components/ChatView.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';

	let sessionId = $derived(page.params.id);
</script>

{#if sessionId}
	<!--
		Keep ChatView mounted across /session/[id] navigations so in-flight
		streaming UI (markdown, scroll state) is not torn down when switching
		back to a session that is still responding. ChatView handles per-session
		init via bindSession + activatedSessionId effect.
	-->
	<div class="session-view">
		<ChatView {sessionId} bootMessage={shellStore.bootMessage} />
	</div>
{/if}

<style>
	.session-view {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-height: 0;
		width: 100%;
	}
</style>
