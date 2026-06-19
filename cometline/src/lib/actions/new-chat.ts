import { goto } from '$app/navigation';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

/** Reset to the hero new-chat screen, same as the sidebar New Chat controls. */
export function startNewChat() {
	const currentSessionId = sessionStore.current?.id ?? chatStore.sessionID;
	if (currentSessionId) {
		const pending = sessionStore.takePendingMessage(currentSessionId);
		if (pending) {
			void chatStore
				.send(
					currentSessionId,
					{ text: pending.text, images: pending.images, filePaths: pending.filePaths },
					{ skipUser: false }
				)
				.catch(() => {});
		}
	}
	const committedWorkspace = shellStore.sidebarOrderWorkspacePath;
	if (shellStore.workspacePath !== committedWorkspace) {
		void window.electronAPI?.setWorkspacePath?.(committedWorkspace);
		shellStore.setWorkspacePath(committedWorkspace);
	}
	sessionStore.selectSession(null);
	chatStore.detachActiveSession();
	shellStore.centerComposer();
	void goto('/');
}
