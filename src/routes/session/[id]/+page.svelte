<script lang="ts">
	import { fade } from 'svelte/transition';
	import { page } from '$app/state';
	import ChatView from '$lib/components/ChatView.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';

	let sessionId = $derived(page.params.id);
	const SESSION_SWITCH = { duration: 140 }; /* keep in sync with --duration-session-switch */
	const SESSION_SWITCH_OUT = { duration: 0 };
</script>

{#if sessionId}
	<!--
		SvelteKit reuses this page component across /session/[id] navigations, so
		key ChatView on the id to remount it per session. ChatView then runs its
		own per-session init (select session, consume pending message) on mount.
	-->
	{#key sessionId}
		<div class="session-view" in:fade={SESSION_SWITCH} out:fade={SESSION_SWITCH_OUT}>
			<ChatView {sessionId} bootMessage={shellStore.bootMessage} />
		</div>
	{/key}
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
