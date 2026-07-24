import { shellStore } from '$lib/stores/shell.svelte';

/** Opens a workspace-relative file in the side panel preview for the active session. */
export function openWorkspaceFilePreview(relativePath: string): void {
	const clean = relativePath.trim();
	if (!clean) return;
	shellStore.openFilePreviewForActive(clean);
}
