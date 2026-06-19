import { shellStore } from '$lib/stores/shell.svelte';
import type { Session } from '$lib/types';

/** Commit sidebar group ordering to a workspace (triggers reorder + flip). */
export function commitSidebarWorkspace(path: string) {
	if (!path || path === shellStore.sidebarOrderWorkspacePath) return;
	shellStore.setSidebarOrderWorkspacePath(path);
}

export function commitSidebarWorkspaceForSession(session: Session | null | undefined) {
	if (!session?.workspace_path) return;
	commitSidebarWorkspace(session.workspace_path);
}
