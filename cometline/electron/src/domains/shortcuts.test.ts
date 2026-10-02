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

function createSettingsShortcutHandler(
	shortcutCaptureActive = false,
	platform?: string,
	shortcuts = normalizeKeyboardShortcuts(undefined)
) {
	let handler: ((event: Event, input: Input) => void) | undefined;
	const hideSettingsWindow = vi.fn();
	const send = vi.fn();
	const webContents = {
		isDestroyed: () => false,
		send,
		on: vi.fn((event: string, listener: (event: Event, input: Input) => void) => {
			if (event === 'before-input-event') handler = listener;
		})
	} as unknown as WebContents;
	const coordinator = createShortcutCoordinator({
		context: {
			getShortcutCaptureActive: () => shortcutCaptureActive
		} as unknown as ShellWindowContext,
		getShortcuts: () => shortcuts,
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
		hideSettingsWindow,
		platform
	});

	coordinator.attachSettingsWindowShortcuts(webContents);
	return { getHandler: () => handler, hideSettingsWindow, send };
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

	it('forwards macOS Command+Enter so the page can capture and send', () => {
		const { getHandler, send } = createSettingsShortcutHandler(true, 'darwin');
		const event = { preventDefault: vi.fn() } as unknown as Event;

		getHandler()?.(event, { ...escapeInput(), key: 'Enter', code: 'Enter', meta: true });

		expect(event.preventDefault).toHaveBeenCalledOnce();
		expect(send).toHaveBeenCalledWith('cometline:command-enter', {
			purpose: 'capture',
			shift: false,
			alt: false,
			control: false
		});
	});

	it('submits when Command+Enter is the send shortcut', () => {
		const shortcuts = normalizeKeyboardShortcuts(undefined);
		shortcuts.sendMessage = { key: 'Enter', command: true };
		const { getHandler, send } = createSettingsShortcutHandler(false, 'darwin', shortcuts);
		const event = { preventDefault: vi.fn() } as unknown as Event;

		getHandler()?.(event, { ...escapeInput(), key: 'Enter', code: 'Enter', meta: true });

		expect(send).toHaveBeenCalledWith('cometline:command-enter', {
			purpose: 'submit',
			shift: false,
			alt: false,
			control: false
		});
	});

	it('does not submit Command+Enter when send is plain Enter', () => {
		const { getHandler, send } = createSettingsShortcutHandler(false, 'darwin');
		const event = { preventDefault: vi.fn() } as unknown as Event;

		getHandler()?.(event, { ...escapeInput(), key: 'Enter', code: 'Enter', meta: true });

		expect(event.preventDefault).toHaveBeenCalledOnce();
		expect(send).not.toHaveBeenCalled();
	});

	it('does not forward Command+Enter off macOS', () => {
		const { getHandler, send } = createSettingsShortcutHandler(false, 'win32');
		const event = { preventDefault: vi.fn() } as unknown as Event;

		getHandler()?.(event, { ...escapeInput(), key: 'Enter', code: 'Enter', meta: true });

		expect(event.preventDefault).not.toHaveBeenCalled();
		expect(send).not.toHaveBeenCalled();
	});
});
