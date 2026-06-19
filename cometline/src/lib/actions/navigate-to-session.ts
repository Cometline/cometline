import { goto } from '$app/navigation';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import type { Session } from '$lib/types';

export interface NavigateToSessionOptions {
	/** When true, reorder sidebar groups immediately. Defaults to true. */
	commitSidebarOrder?: boolean;
}

/** Activate a session; composer follows the session workspace immediately. */
export function navigateToSession(session: Session, options: NavigateToSessionOptions = {}) {
	const commitSidebarOrder = options.commitSidebarOrder ?? true;

	sessionStore.selectSession(session);
	modelStore.selectFromSession(session);

	if (session.workspace_path && session.workspace_path !== shellStore.workspacePath) {
		void window.electronAPI?.setWorkspacePath?.(session.workspace_path);
		shellStore.setWorkspacePath(session.workspace_path);
	}

	if (commitSidebarOrder && session.workspace_path) {
		shellStore.setSidebarOrderWorkspacePath(session.workspace_path);
	}

	void goto(`/session/${session.id}`);
}
