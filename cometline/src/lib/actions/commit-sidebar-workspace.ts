import { shellStore } from '#lib/stores/shell.svelte.js';
import { isDiscordSession } from '#lib/sessions/group-by-workspace.js';
import type { Session } from '#lib/types.js';

/** Commit sidebar group ordering to a workspace (triggers reorder + flip). */
export function commitSidebarWorkspace(path: string) {
	if (!path || path === shellStore.sidebarOrderWorkspacePath) return;
	shellStore.setSidebarOrderWorkspacePath(path);
	shellStore.setSidebarOrderDiscordActive(false);
}

export function commitSidebarWorkspaceForSession(session: Session | null | undefined) {
	if (!session?.workspace_path || session.pinned) return;
	if (session.workspace_path !== shellStore.sidebarOrderWorkspacePath) {
		shellStore.setSidebarOrderWorkspacePath(session.workspace_path);
	}
	shellStore.setSidebarOrderDiscordActive(isDiscordSession(session));
}
