import { getActiveSessionId } from '#lib/active-session.js';
import type { ShellFocusStore } from './focus.svelte';
import { syncWorkspacePanelOpen, type WorkspacePanelStore } from './workspace-panel.svelte';

type TerminalPanelDeps = {
	panel: WorkspacePanelStore;
	focus: ShellFocusStore;
};

/** Terminal surface visibility and the pane focus that follows active-session changes. */
export function createTerminalPanel({ panel, focus }: TerminalPanelDeps) {
	function openTerminalPanel() {
		const sessionId = getActiveSessionId();
		if (!sessionId) return false;
		panel.showTerminalSurface(sessionId);
		focus.setFocusedPane('terminal');
		focus.bumpTerminalFocus();
		syncWorkspacePanelOpen(true);
		return true;
	}

	return {
		onActiveSessionChange() {
			const sessionId = getActiveSessionId();
			if (
				sessionId &&
				panel.surfaceFor(sessionId) === 'terminal' &&
				panel.isTerminalVisible(sessionId)
			) {
				focus.setFocusedPane('terminal');
			} else {
				focus.setFocusedPane('chat');
			}
			panel.syncOpenForActive();
		},
		openTerminalPanel,
		requestTerminalFocus() {
			// openTerminalPanel already bumps terminalFocusRequestId.
			openTerminalPanel();
		},
		closeTerminalPanelForSession(sessionId: string) {
			panel.removeTerminal(sessionId);
			if (getActiveSessionId() !== sessionId || panel.activeSurface() !== 'terminal') return;
			focus.requestComposerFocus();
			syncWorkspacePanelOpen(false);
		}
	};
}
