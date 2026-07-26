import type { Event, Input, WebContents } from 'electron';
import { describe, expect, it, vi } from 'vitest';

vi.mock('electron', () => ({
	app: { isReady: vi.fn(() => true) },
	globalShortcut: {
		register: vi.fn(() => true),
		unregister: vi.fn(),
		unregisterAll: vi.fn()
	}
}));

import { normalizeKeyboardShortcuts } from '../../../src/lib/keyboard-shortcuts.js';
import type { ShellWindowContext } from './runtime-context.js';
import { createShortcutCoordinator } from './shortcuts.js';

function createSettingsShortcutHandler(shortcutCaptureActive = false) {
	let handler: ((event: Event, input: Input) => void) | undefined;
	const hideSettingsWindow = vi.fn();
	const webContents = {
		on: vi.fn((event: string, listener: (event: Event, input: Input) => void) => {
			if (event === 'before-input-event') handler = listener;
		})
	} as unknown as WebContents;
	const coordinator = createShortcutCoordinator({
		context: {
			getShortcutCaptureActive: () => shortcutCaptureActive
		} as unknown as ShellWindowContext,
		getShortcuts: () => normalizeKeyboardShortcuts(undefined),
		routeSignals: {
			closeInbox: vi.fn(),
			closeWorkspacePanel: vi.fn(),
			requestCloseWindow: vi.fn(),
			requestReload: vi.fn(),
			sendNavigateSession: vi.fn(),
			sendShortcutAction: vi.fn()
		},
		showSettingsWindow: vi.fn(async () => undefined),
		toggleMiniWindow: vi.fn(async () => undefined),
		hideMainWindow: vi.fn(),
		hideMiniWindow: vi.fn(),
		hideSettingsWindow
	});

	coordinator.attachSettingsWindowShortcuts(webContents);
	return { getHandler: () => handler, hideSettingsWindow };
}

function escapeInput(): Input {
	return {
		type: 'keyDown',
		key: 'Escape',
		code: 'Escape',
		isAutoRepeat: false,
		isComposing: false,
		shift: false,
		control: false,
		alt: false,
		meta: false,
		location: 0,
		modifiers: []
	};
}

describe('settings window shortcuts', () => {
	it('hides the settings window for the configured close shortcut', () => {
		const { getHandler, hideSettingsWindow } = createSettingsShortcutHandler();
		const event = { preventDefault: vi.fn() } as unknown as Event;

		getHandler()?.(event, escapeInput());

		expect(event.preventDefault).toHaveBeenCalledOnce();
		expect(hideSettingsWindow).toHaveBeenCalledOnce();
	});

	it('does not close while a shortcut is being captured', () => {
		const { getHandler, hideSettingsWindow } = createSettingsShortcutHandler(true);
		const event = { preventDefault: vi.fn() } as unknown as Event;

		getHandler()?.(event, escapeInput());

		expect(event.preventDefault).not.toHaveBeenCalled();
		expect(hideSettingsWindow).not.toHaveBeenCalled();
	});
});
