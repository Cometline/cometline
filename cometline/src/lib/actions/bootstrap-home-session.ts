import { startNewChat } from '$lib/actions/new-chat';
import { connectionState } from '$lib/stores/runtime.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

export type BootstrapHomeSessionDeps = {
	connectionStatus: () => typeof connectionState.status;
	workspacePath: () => string;
	startNewChat: () => Promise<unknown>;
};

const defaultDeps: BootstrapHomeSessionDeps = {
	connectionStatus: () => connectionState.status,
	workspacePath: () => shellStore.defaultWorkspacePath || shellStore.workspacePath,
	startNewChat
};

/**
 * When the home route is ready (sidecar up + workspace set), create a persisted
 * session and navigate to it — same as New Chat. Returns false when skipped.
 */
export async function bootstrapHomeSession(
	deps: BootstrapHomeSessionDeps = defaultDeps
): Promise<boolean> {
	if (deps.connectionStatus() !== 'ready') return false;
	const workspace = deps.workspacePath().trim();
	if (!workspace || workspace === '/') return false;
	await deps.startNewChat();
	return true;
}
