import type { Session } from '$lib/types';

/**
 * Picks the session that should become active after deleting `deletedId`.
 * Prefers the next row in the given ordered list; falls back to the previous.
 */
export function nextSessionAfterDelete(
	deletedId: string,
	orderedSessions: readonly Session[]
): Session | null {
	const index = orderedSessions.findIndex((session) => session.id === deletedId);
	const remaining = orderedSessions.filter((session) => session.id !== deletedId);
	if (remaining.length === 0) return null;
	if (index < 0) return remaining[0] ?? null;
	return remaining[Math.min(index, remaining.length - 1)] ?? null;
}
