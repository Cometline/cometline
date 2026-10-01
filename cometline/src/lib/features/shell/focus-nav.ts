/**
 * Cmd+[ / Cmd+] use the workspace panel navigation stack (and webview history when
 * on a URL) whenever the workspace panel is open and mounted — not only when the
 * web pane has keyboard focus. While its lazy chunk loads, session history remains
 * available instead of swallowing the shortcut.
 */
export function shouldUseWorkspacePanelHistory(
	workspacePanelOpen: boolean,
	workspacePanelReady: boolean
): boolean {
	return workspacePanelOpen && workspacePanelReady;
}
