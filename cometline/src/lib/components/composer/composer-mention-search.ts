export function shouldRunMentionServerSearch(
	truncated: boolean,
	query: string
): boolean {
	return truncated && query.trim().length > 0;
}

/** Prefer warm-cache matches; fall back to server results only when the cache has none. */
export function resolveMentionSourcePaths(
	localMatches: string[] | null | undefined,
	serverMatches: string[] | null | undefined,
	serverQuery: string,
	query: string,
	useServerSearch: boolean
): string[] {
	const local = localMatches ?? [];
	if (!useServerSearch || serverQuery !== query.trim()) return local;
	return serverMatches ?? local;
}
