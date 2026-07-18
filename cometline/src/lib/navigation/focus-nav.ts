/**
 * Cmd+[ / Cmd+] use the web panel navigation stack (and webview history when
 * on a URL) whenever the web panel is open — not only when the web pane has
 * keyboard focus — so session history does not steal the shortcut.
 */
export function shouldUseWebPanelHistory(webPanelOpen: boolean): boolean {
	return webPanelOpen;
}
