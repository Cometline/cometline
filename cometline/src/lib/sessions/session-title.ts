/** Shown in the sidebar/title bar when a session has no persisted title yet. */
export const DEFAULT_SESSION_TITLE = 'New Chat';

/** Display title for a session; empty titles render as {@link DEFAULT_SESSION_TITLE}. */
export function sessionDisplayTitle(title: string | null | undefined): string {
	const trimmed = title?.trim();
	return trimmed || DEFAULT_SESSION_TITLE;
}
