import { shellStore } from '$lib/stores/shell.svelte';
import type { FileRevealRange } from '$lib/workspace/workspace-panel-state';

/** Opens a workspace-relative file in the side panel preview for the active session. */
export function openWorkspaceFilePreview(
	relativePath: string,
	reveal?: FileRevealRange | null
): void {
	const clean = relativePath.trim();
	if (!clean) return;
	void shellStore.openFilePreviewForActive(clean, reveal ?? undefined);
}
