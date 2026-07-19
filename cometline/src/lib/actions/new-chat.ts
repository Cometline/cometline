import { goto } from '$app/navigation';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { createNewSession } from '$lib/actions/create-new-session';

/** Create and open a persisted session, same as the sidebar New Chat controls. */
export async function startNewChat() {
	const currentSessionId = sessionStore.current?.id ?? chatStore.sessionID;
	if (currentSessionId) {
		const pending = sessionStore.takePendingMessage(currentSessionId);
		if (pending) {
			void chatStore
				.send(
					currentSessionId,
					{
						text: pending.text,
						images: pending.images,
						filePaths: pending.filePaths,
						webContexts: pending.webContexts
					},
					{ skipUser: false }
				)
				.catch(() => {});
		}
	}
	const session = await createNewSession();
	await goto(`/session/${session.id}`);
}
