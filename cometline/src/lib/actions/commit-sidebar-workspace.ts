import { shellStore } from '$lib/stores/shell.svelte';
import { isDiscordSession } from '$lib/sessions/group-by-workspace';
import type { Session } from '$lib/types';

/** Commit sidebar group ordering to a workspace (triggers reorder + flip). */
export function commitSidebarWorkspace(path: string) {
	if (!path || path === shellStore.sidebarOrderWorkspacePath) return;
	shellStore.setSidebarOrderWorkspacePath(path);
	shellStore.setSidebarOrderDiscordActive(false);
}

export function commitSidebarWorkspaceForSession(session: Session | null | undefined) {
	if (!session?.workspace_path) return;
	if (session.workspace_path !== shellStore.sidebarOrderWorkspacePath) {
		shellStore.setSidebarOrderWorkspacePath(session.workspace_path);
	}
	shellStore.setSidebarOrderDiscordActive(isDiscordSession(session));
}
