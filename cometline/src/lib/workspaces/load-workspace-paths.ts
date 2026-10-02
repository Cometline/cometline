import { listWorkspaces } from '#lib/client/cometmind.js';

export type WorkspacePathSource = {
	listRecentWorkspaces?: () => Promise<string[]>;
	filterExistingWorkspacePaths?: (paths: string[]) => Promise<string[]>;
	listRegisteredWorkspaces?: () => Promise<Array<{ path: string; session_count: number }>>;
};

export function mergeWorkspacePaths(
	recent: string[],
	registered: Array<{ path: string; session_count: number }>,
	currentPath?: string
): { paths: string[]; sessionCountByPath: Map<string, number> } {
	const sessionCountByPath = new Map<string, number>();
	for (const ws of registered) {
		const clean = ws.path.trim();
		if (!clean) continue;
		sessionCountByPath.set(clean, ws.session_count);
	}

	const seen = new Set<string>();
	const paths: string[] = [];
	const add = (path: string) => {
		const clean = path.trim();
		if (!clean || seen.has(clean)) return;
		seen.add(clean);
		paths.push(clean);
	};

	for (const path of recent) add(path);
	if (currentPath) add(currentPath);
	for (const ws of registered) add(ws.path);

	return { paths, sessionCountByPath };
}

export async function loadWorkspacePaths(
	currentPath?: string,
	source: WorkspacePathSource = {}
): Promise<{ paths: string[]; sessionCountByPath: Map<string, number> }> {
	const listRecent =
		source.listRecentWorkspaces ??
		(async () => (await window.electronAPI?.listRecentWorkspaces?.()) ?? []);
	const filterExisting =
		source.filterExistingWorkspacePaths ??
		(async (paths: string[]) =>
			(await window.electronAPI?.filterExistingWorkspacePaths?.(paths)) ?? paths);
	const listRegistered =
		source.listRegisteredWorkspaces ??
		(async () => {
			const workspaces = await listWorkspaces().catch(() => []);
			return workspaces.map((ws) => ({
				path: ws.path,
				session_count: ws.session_count
			}));
		});

	const [recent, registered] = await Promise.all([listRecent(), listRegistered()]);
	const merged = mergeWorkspacePaths(recent, registered, currentPath);
	const paths = await filterExisting(merged.paths);
	return { paths, sessionCountByPath: merged.sessionCountByPath };
}
