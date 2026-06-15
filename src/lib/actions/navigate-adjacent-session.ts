import { goto } from '$app/navigation';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';

/** Move to the previous or next chat in the sidebar list (newest first). */
export function navigateAdjacentSession(direction: 'prev' | 'next') {
	const sessions = sessionStore.sessions;
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

	const session = sessions[nextIndex];
	sessionStore.selectSession(session);
	modelStore.selectFromSession(session);
	void goto(`/session/${session.id}`);
}
