/** Matches `--sidebar-auto-collapse-max` (default 900px). */
export function narrowViewportQuery(): MediaQueryList {
	const max = getComputedStyle(document.documentElement)
		.getPropertyValue('--sidebar-auto-collapse-max')
		.trim();
	return window.matchMedia(`(max-width: ${max || '900px'})`);
}

export function isNarrowViewport(): boolean {
	return narrowViewportQuery().matches;
}

/** Run when the narrow-viewport breakpoint is crossed (e.g. window resize). */
export function subscribeNarrowViewport(
	handler: (narrow: boolean) => void
): () => void {
	const query = narrowViewportQuery();
	const listener = (event: MediaQueryListEvent) => handler(event.matches);
	query.addEventListener('change', listener);
	return () => query.removeEventListener('change', listener);
}
