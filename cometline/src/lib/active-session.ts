import { browser } from '$app/env';
import { currentPathname } from '#lib/routes/session-route.js';
import { sessionStore } from '#lib/stores/session.svelte.js';

export function sessionIdFromPathname(pathname: string): string | null {
	const match = pathname.match(/^\/(?:mini\/)?session\/([^/?#]+)/);
	return match?.[1] ?? null;
}

/** Session id from store, or from /session/:id when the store has not synced yet. */
export function getActiveSessionId(): string | null {
	if (sessionStore.current?.id) return sessionStore.current.id;
	if (!browser) return null;
	return sessionIdFromPathname(currentPathname());
}
