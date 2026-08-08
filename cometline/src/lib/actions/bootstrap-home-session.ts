import { startNewChat } from '$lib/actions/new-chat';
import { navigateToSession } from '$lib/actions/navigate-to-session';
import { connectionState } from '$lib/stores/runtime.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { sessionVisitHistory } from '$lib/stores/session-visit-history.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import type { Session } from '$lib/types';

export type BootstrapHomeSessionDeps = {
	connectionStatus: () => typeof connectionState.status;
	workspacePath: () => string;
	sessionsLoaded: () => boolean;
	sessions: () => readonly Session[];
	mostRecentSessionId: (exists: (sessionId: string) => boolean) => string | null;
	navigateToSession: (session: Session) => Promise<unknown>;
	startNewChat: () => Promise<unknown>;
};

const defaultDeps: BootstrapHomeSessionDeps = {
	connectionStatus: () => connectionState.status,
	workspacePath: () => shellStore.defaultWorkspacePath || shellStore.workspacePath,
	sessionsLoaded: () => sessionStore.loaded,
	sessions: () => sessionStore.sessions,
	mostRecentSessionId: (exists) => sessionVisitHistory.mostRecent(exists),
	navigateToSession,
	startNewChat
};

/**
 * When the home route is ready, restore the most recently visited session. If
 * no sessions exist, create and navigate to one — same as New Chat.
 */
export async function bootstrapHomeSession(
	deps: BootstrapHomeSessionDeps = defaultDeps
): Promise<boolean> {
	if (deps.connectionStatus() !== 'ready') return false;
	const workspace = deps.workspacePath().trim();
	if (!workspace || workspace === '/') return false;
	if (!deps.sessionsLoaded()) return false;

	const sessions = deps.sessions();
	const sessionExists = (sessionId: string) => sessions.some((session) => session.id === sessionId);
	const recentSessionId = deps.mostRecentSessionId(sessionExists);
	const recentSession = recentSessionId
		? sessions.find((session) => session.id === recentSessionId)
		: undefined;
	const fallbackSession = sessions.reduce<Session | undefined>((latest, session) => {
		if (!latest || session.updated_at > latest.updated_at) return session;
		return latest;
	}, undefined);

	if (recentSession ?? fallbackSession) {
		await deps.navigateToSession(recentSession ?? fallbackSession!);
		return true;
	}

	await deps.startNewChat();
	return true;
}
