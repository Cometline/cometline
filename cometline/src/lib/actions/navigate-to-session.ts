import { goto } from '$app/navigation';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { sessionVisitHistory } from '$lib/stores/session-visit-history.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { isDiscordSession } from '$lib/sessions/group-by-workspace';
import type { Session } from '$lib/types';

export interface NavigateToSessionOptions {
	/** When true, reorder sidebar groups immediately. Defaults to true for unpinned sessions. */
	commitSidebarOrder?: boolean;
	/** When true, skip recording this activation in the visit history stack. */
	fromHistory?: boolean;
}

/** Activate a session; composer follows the session workspace immediately. */
export function navigateToSession(session: Session, options: NavigateToSessionOptions = {}) {
	const commitSidebarOrder = options.commitSidebarOrder ?? !session.pinned;

	sessionStore.selectSession(session);
	modelStore.selectFromSession(session);

	if (session.workspace_path && session.workspace_path !== shellStore.workspacePath) {
		shellStore.setActiveWorkspacePath(session.workspace_path);
	}

	if (commitSidebarOrder) {
		if (session.workspace_path) {
			shellStore.setSidebarOrderWorkspacePath(session.workspace_path);
		}
		shellStore.setSidebarOrderDiscordActive(isDiscordSession(session));
	}

	if (!options.fromHistory) {
		sessionVisitHistory.recordVisit(session.id);
	}

	shellStore.requestComposerFocus(session.id);
	void goto(`/session/${session.id}`);
}
