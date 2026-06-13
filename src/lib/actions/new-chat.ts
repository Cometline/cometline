import { goto } from '$app/navigation';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

/** Reset to the hero new-chat screen, same as the sidebar New Chat controls. */
export function startNewChat() {
	sessionStore.selectSession(null);
	chatStore.clear();
	shellStore.centerComposer();
	void goto('/');
}
