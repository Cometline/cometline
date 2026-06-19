import type { Session } from '$lib/types';

export interface WorkspaceSessionGroup {
	workspacePath: string;
	label: string;
	sessions: Session[];
}

/** Returns the final path segment used as a short directory label. */
export function workspaceLabel(path: string): string {
	const parts = path.split(/[/\\]/).filter(Boolean);
	return parts[parts.length - 1] || path;
}

/**
 * Groups sessions by their workspace path. Groups are ordered with the active
 * workspace first, then by the most recently updated session in each group.
 * Sessions within a group preserve their incoming order (recent first).
 */
export function groupSessionsByWorkspace(
	sessions: Session[],
	activeWorkspacePath = ''
): WorkspaceSessionGroup[] {
	const groups = new Map<string, WorkspaceSessionGroup>();
	const mostRecent = new Map<string, number>();

	for (const session of sessions) {
		const path = session.workspace_path;
		let group = groups.get(path);
		if (!group) {
			group = { workspacePath: path, label: workspaceLabel(path), sessions: [] };
			groups.set(path, group);
		}
		group.sessions.push(session);
		const updatedAt = session.updated_at ?? 0;
		mostRecent.set(path, Math.max(mostRecent.get(path) ?? 0, updatedAt));
	}

	return Array.from(groups.values()).sort((a, b) => {
		if (a.workspacePath === activeWorkspacePath) return -1;
		if (b.workspacePath === activeWorkspacePath) return 1;
		return (mostRecent.get(b.workspacePath) ?? 0) - (mostRecent.get(a.workspacePath) ?? 0);
	});
}

/** Flatten sessions in sidebar group order for keyboard navigation. */
export function flattenSessionsInSidebarOrder(
	sessions: Session[],
	orderWorkspacePath = ''
): Session[] {
	return groupSessionsByWorkspace(sessions, orderWorkspacePath).flatMap((group) => group.sessions);
}
