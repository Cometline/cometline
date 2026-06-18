import { getActiveSessionId } from '$lib/active-session';
import { shellStore } from '$lib/stores/shell.svelte';

/** Opens a workspace-relative file in the side panel preview. */
export function openWorkspaceFilePreview(relativePath: string): void {
	const clean = relativePath.trim();
	if (!clean) return;
	const sessionId = getActiveSessionId();
	if (!sessionId) return;
	shellStore.openFilePreview(clean, sessionId);
}
