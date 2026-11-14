import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { chatStore } from '#lib/stores/chat.svelte.js';
import { sessionStore } from '#lib/stores/session.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { appToastStore } from '#lib/stores/app-toasts.svelte.js';
import { createNewSession } from '#lib/actions/create-new-session.js';

export function reportNewChatStartError(error: unknown) {
	const detail = error instanceof Error ? error.message : String(error);
	console.error('startNewChat failed:', error);
	appToastStore.error('Could not start new chat', detail);
}

/** Create and open a persisted session, same as the sidebar New Chat controls.
 * Pass `workspacePath` to pin the new session to a sidebar group instead of the default workspace.
 */
export async function startNewChat(workspacePath?: string) {
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
						webContexts: pending.webContexts,
						agentMode: pending.agentMode
					},
					{ skipUser: false }
				)
				.catch(() => {});
		}
	}
	// Creating the next persisted session may wait on the sidecar. Unbind now so
	// the current turn queue can keep draining without the old view staying active.
	const session = await createNewSession(workspacePath);
	chatStore.detachActiveSession();
	await goto(resolve('/session/[id]', { id: session.id }));
	shellStore.requestComposerFocus(session.id);
}
