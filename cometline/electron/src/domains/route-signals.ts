import type { ShortcutAction } from '../../../src/lib/keyboard-shortcuts.js';
import { EVENT_CHANNELS, type EventChannel } from '../shared/ipc-channels.js';
import type { ShellWindowContext } from './runtime-context.js';

/** Sends renderer route and shortcut signals through the current main window. */
export function createRouteSignals(context: ShellWindowContext) {
	function send(channel: EventChannel, ...args: unknown[]) {
		const mainWindow = context.getMainWindow();
		if (!mainWindow || mainWindow.isDestroyed()) return;
		mainWindow.webContents.send(channel, ...args);
	}

	return {
		closeInbox: () => send(EVENT_CHANNELS.onCloseInbox),
		closeWorkspacePanel: () => send(EVENT_CHANNELS.onCloseWorkspacePanel),
		requestCloseWindow: () => send(EVENT_CHANNELS.onRequestCloseWindow),
		requestReload: () => send(EVENT_CHANNELS.onRequestReload),
		sendNavigateSession: (direction: 'prev' | 'next') =>
			send(EVENT_CHANNELS.onNavigateSession, direction),
		sendShortcutAction: (action: ShortcutAction) =>
			send(EVENT_CHANNELS.onShortcutAction, action)
	};
}
