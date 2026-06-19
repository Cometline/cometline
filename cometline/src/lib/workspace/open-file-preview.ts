import { shellStore } from '$lib/stores/shell.svelte';

/** Opens a workspace-relative file in the side panel preview. */
export function openWorkspaceFilePreview(relativePath: string): void {
	const clean = relativePath.trim();
	if (!clean) return;
	// Works on the home route too: with no session yet the panel opens under a
	// draft key and is migrated onto the real session on first send.
	shellStore.openFilePreviewForActive(clean);
}
