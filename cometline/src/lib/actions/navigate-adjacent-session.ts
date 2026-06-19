import { navigateToSession } from '$lib/actions/navigate-to-session';
import { flattenSessionsInSidebarOrder } from '$lib/sessions/group-by-workspace';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

/** Move to the previous or next chat in committed sidebar order. */
export function navigateAdjacentSession(direction: 'prev' | 'next') {
	const sessions = flattenSessionsInSidebarOrder(
		sessionStore.sessions,
		shellStore.sidebarOrderWorkspacePath
	);
	if (sessions.length === 0) return;

	const currentId = sessionStore.current?.id ?? null;
	let nextIndex: number;

	if (!currentId) {
		if (direction === 'next') {
			nextIndex = 0;
		} else {
			return;
		}
	} else {
		const currentIndex = sessions.findIndex((session) => session.id === currentId);
		if (currentIndex === -1) return;
		nextIndex = direction === 'prev' ? currentIndex - 1 : currentIndex + 1;
	}

	if (nextIndex < 0 || nextIndex >= sessions.length) return;

	navigateToSession(sessions[nextIndex], { commitSidebarOrder: false });
}
