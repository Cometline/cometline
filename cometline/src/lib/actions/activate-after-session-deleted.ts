import { startNewChat } from '$lib/actions/new-chat';
import { navigateToSession } from '$lib/actions/navigate-to-session';
import { flattenSessionsInSidebarOrder } from '$lib/sessions/group-by-workspace';
import { nextSessionAfterDelete } from '$lib/sessions/next-session-after-delete';
import { chatStore } from '$lib/stores/chat.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import type { Session } from '$lib/types';

export type ActivateAfterSessionDeletedOptions = {
	navigate?: (session: Session) => void | Promise<void>;
	createEmpty?: () => Promise<unknown>;
};

/**
 * After deleting the active session, select the next existing chat in sidebar
 * order. If none remain, start a new persisted session (same as New Chat).
 *
 * `sessionsBeforeDelete` should be the list snapshot before remove. Any ids that
 * are no longer in `sessionStore.sessions` (e.g. bulk retention) are skipped
 * when choosing a neighbor.
 */
export async function activateAfterSessionDeleted(
	deletedId: string,
	sessionsBeforeDelete: readonly Session[],
	options: ActivateAfterSessionDeletedOptions = {}
): Promise<void> {
	chatStore.clear();
	const remainingIds = new Set(sessionStore.sessions.map((session) => session.id));
	const ordered = flattenSessionsInSidebarOrder(
		[...sessionsBeforeDelete],
		shellStore.sidebarOrderWorkspacePath,
		shellStore.sidebarOrderDiscordActive
	).filter((session) => session.id === deletedId || remainingIds.has(session.id));
	const next = nextSessionAfterDelete(deletedId, ordered);
	if (next) {
		await (options.navigate ?? navigateToSession)(next);
		return;
	}
	await (options.createEmpty ?? startNewChat)();
}

/** Snapshot of the session list before `removeSession` mutates the store. */
export function sessionsSnapshot(): Session[] {
	return [...sessionStore.sessions];
}
