/**
 * Cmd+[ / Cmd+] use the workspace panel navigation stack (and webview history when
 * on a URL) whenever the workspace panel is open — not only when the web pane has
 * keyboard focus — so session history does not steal the shortcut.
 */
export function shouldUseWorkspacePanelHistory(workspacePanelOpen: boolean): boolean {
	return workspacePanelOpen;
}
