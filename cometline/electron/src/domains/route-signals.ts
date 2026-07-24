import type { ShortcutAction } from '../../../src/lib/keyboard-shortcuts.js';
import type { ShellWindowContext } from './runtime-context.js';

/** Sends renderer route and shortcut signals through the current main window. */
export function createRouteSignals(context: ShellWindowContext) {
	function send(channel: string, ...args: unknown[]) {
		const mainWindow = context.getMainWindow();
		if (!mainWindow || mainWindow.isDestroyed()) return;
		mainWindow.webContents.send(channel, ...args);
	}

	return {
		closeInbox: () => send('cometline:close-inbox'),
		closeWorkspacePanel: () => send('cometline:close-workspace-panel'),
		requestCloseWindow: () => send('cometline:request-close-window'),
		requestReload: () => send('cometline:request-reload'),
		sendNavigateSession: (direction: 'prev' | 'next') =>
			send('cometline:navigate-session', direction),
		sendShortcutAction: (action: ShortcutAction) => send('cometline:shortcut-action', action)
	};
}
