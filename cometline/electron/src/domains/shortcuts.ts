import { app, globalShortcut, type Event, type Input, type WebContents } from 'electron';
import {
	type KeyboardShortcuts,
	type ShortcutAction,
	type ShortcutBinding,
	isReloadShortcut
} from '../../../src/lib/keyboard-shortcuts.js';
import type { ShellWindowContext } from './runtime-context.js';

const UNRELIABLE_SHORTCUT_KEYS = new Set(['', 'Process', 'Unidentified', 'Dead']);
const SHORTCUT_CODE_KEY_MAP: Record<string, string> = {
	Comma: ',',
	Period: '.',
	Slash: '/',
	Backslash: '\\',
	Semicolon: ';',
	Quote: "'",
	BracketLeft: '[',
	BracketRight: ']',
	Minus: '-',
	Equal: '=',
	Backquote: '`',
	Space: ' ',
	ArrowUp: 'ArrowUp',
	ArrowDown: 'ArrowDown',
	ArrowLeft: 'ArrowLeft',
	ArrowRight: 'ArrowRight',
	Enter: 'Enter',
	Escape: 'Escape',
	Tab: 'Tab',
	Backspace: 'Backspace',
	Delete: 'Delete'
};

interface RouteSignals {
	closeInbox(): void;
	closeWorkspacePanel(): void;
	requestCloseWindow(): void;
	requestReload(): void;
	sendNavigateSession(direction: 'prev' | 'next'): void;
	sendShortcutAction(action: ShortcutAction): void;
}

interface ShortcutCoordinatorDependencies {
	context: ShellWindowContext;
	getShortcuts(): KeyboardShortcuts;
	routeSignals: RouteSignals;
	showSettingsWindow(): Promise<void>;
	toggleMiniWindow(): Promise<void>;
	hideMainWindow(): void;
	hideMiniWindow(): void;
	hideSettingsWindow(): void;
}

function shortcutKeyMatches(a: string, b: string) {
	return a === b || String(a).toLowerCase() === String(b).toLowerCase();
}

function keyFromInputCode(code: string | undefined) {
	if (!code) return null;
	const letter = code.match(/^Key([A-Z])$/);
	if (letter) return letter[1].toLowerCase();
	const digit = code.match(/^Digit([0-9])$/);
	if (digit) return digit[1];
	const fn = code.match(/^F([1-9]|1[0-9]|2[0-4])$/);
	if (fn) return code.toUpperCase();
	return SHORTCUT_CODE_KEY_MAP[code] ?? null;
}

function shortcutInputKey(input: Input) {
	const codeKey = keyFromInputCode(input.code);
	if (
		codeKey &&
		(input.control ||
			input.meta ||
			input.alt ||
			UNRELIABLE_SHORTCUT_KEYS.has(input.key) ||
			input.isComposing)
	) {
		return codeKey;
	}
	return input.key;
}

function matchesInputShortcut(input: Input, binding: ShortcutBinding | undefined) {
	if (input.type !== 'keyDown' || !binding?.key) return false;
	if (!shortcutKeyMatches(shortcutInputKey(input), binding.key)) return false;

	const expectsCommand = binding.command ?? false;
	if (expectsCommand) {
		if (!(input.control || input.meta)) return false;
		if (binding.ctrl !== true && input.control && input.meta) return false;
		if (binding.alt !== undefined ? binding.alt !== input.alt : input.alt) return false;
		if (binding.shift !== undefined ? binding.shift !== input.shift : input.shift) return false;
		return true;
	}

	if (binding.ctrl !== undefined && binding.ctrl !== input.control) return false;
	if (binding.meta !== undefined && binding.meta !== input.meta) return false;
	if (binding.alt !== undefined && binding.alt !== input.alt) return false;
	if (binding.shift !== undefined && binding.shift !== input.shift) return false;
	return true;
}

export function shortcutBindingToAccelerator(binding: ShortcutBinding | undefined) {
	if (!binding?.key) return '';
	const modifiers: string[] = [];
	if (binding.command) modifiers.push('CommandOrControl');
	if (binding.ctrl) modifiers.push('Control');
	if (binding.meta) modifiers.push('Meta');
	if (binding.alt) modifiers.push('Alt');
	if (binding.shift) modifiers.push('Shift');
	const normalized = String(binding.key).trim();
	const named: Record<string, string> = {
		ArrowUp: 'Up',
		ArrowDown: 'Down',
		ArrowLeft: 'Left',
		ArrowRight: 'Right',
		',': 'Comma',
		'.': 'Period',
		Escape: 'Escape',
		Esc: 'Escape',
		Enter: 'Enter',
		Space: 'Space',
		' ': 'Space'
	};
	const key =
		named[normalized] ??
		(/^F\d{1,2}$/i.test(normalized)
			? normalized.toUpperCase()
			: normalized.length === 1
				? normalized.toUpperCase()
				: normalized);
	if (!key) return '';
	return [...modifiers, key].join('+');
}

/** Matches renderer input, guest input, and the mini-window global shortcut. */
export function createShortcutCoordinator(dependencies: ShortcutCoordinatorDependencies) {
	const {
		context,
		getShortcuts,
		routeSignals,
		showSettingsWindow,
		toggleMiniWindow,
		hideMainWindow,
		hideMiniWindow,
		hideSettingsWindow
	} = dependencies;
	let registeredMiniWindowShortcut = '';

	function handleDarwinCloseWindowShortcut(
		event: Event,
		input: Input,
		onCloseWindow: () => void,
		closesMainWindow = false
	) {
		const isCloseWindowShortcut =
			process.platform === 'darwin' &&
			input.type === 'keyDown' &&
			input.meta &&
			!input.control &&
			!input.alt &&
			!input.shift &&
			input.key?.toLowerCase() === 'w';
		if (!isCloseWindowShortcut) return false;
		event.preventDefault();
		if (closesMainWindow) {
			if (context.getInboxOpen()) {
				routeSignals.closeInbox();
				return true;
			}
			if (context.getWorkspacePanelOpen()) {
				routeSignals.closeWorkspacePanel();
				return true;
			}
			routeSignals.requestCloseWindow();
			return true;
		}
		onCloseWindow();
		return true;
	}

	function handleReloadShortcut(event: Event, input: Input) {
		if (input.type !== 'keyDown') return false;
		if (
			!isReloadShortcut({
				key: input.key,
				code: input.code,
				meta: input.meta,
				control: input.control,
				alt: input.alt,
				shift: input.shift,
				isComposing: input.isComposing
			})
		) {
			return false;
		}
		event.preventDefault();
		routeSignals.requestReload();
		return true;
	}

	function attachWindowShortcuts(
		webContents: WebContents,
		options: {
			onCloseWindow: () => void;
			includeSessionNavigation?: boolean;
			includeSettingsShortcut?: boolean;
			includeCloseSettingsShortcut?: boolean;
			closesMainWindow?: boolean;
			includeReloadShortcut?: boolean;
		}
	) {
		webContents.on('before-input-event', (event, input) => {
			if (
				handleDarwinCloseWindowShortcut(
					event,
					input,
					options.onCloseWindow,
					options.closesMainWindow
				)
			) {
				return;
			}

			if (options.includeReloadShortcut && handleReloadShortcut(event, input)) {
				return;
			}

			const shortcuts = getShortcuts();
			if (
				options.includeCloseSettingsShortcut &&
				!context.getShortcutCaptureActive() &&
				matchesInputShortcut(input, shortcuts.closeSettings)
			) {
				event.preventDefault();
				options.onCloseWindow();
				return;
			}
			if (
				options.includeSettingsShortcut &&
				matchesInputShortcut(input, shortcuts.openSettings)
			) {
				event.preventDefault();
				void showSettingsWindow();
				return;
			}

			if (!options.includeSessionNavigation) return;
			if (context.getShortcutCaptureActive() || context.getSessionNavigationSuspended())
				return;
			if (matchesInputShortcut(input, shortcuts.previousSession)) {
				event.preventDefault();
				routeSignals.sendNavigateSession('prev');
				return;
			}
			if (matchesInputShortcut(input, shortcuts.nextSession)) {
				event.preventDefault();
				routeSignals.sendNavigateSession('next');
			}
		});
	}

	function attachMainWindowShortcuts(webContents: WebContents) {
		attachWindowShortcuts(webContents, {
			onCloseWindow: hideMainWindow,
			includeSessionNavigation: true,
			closesMainWindow: true,
			includeReloadShortcut: true
		});
	}

	function attachMiniWindowShortcuts(webContents: WebContents) {
		attachWindowShortcuts(webContents, {
			onCloseWindow: hideMiniWindow,
			includeSettingsShortcut: true
		});
	}

	function attachSettingsWindowShortcuts(webContents: WebContents) {
		attachWindowShortcuts(webContents, {
			onCloseWindow: hideSettingsWindow,
			includeCloseSettingsShortcut: true
		});
	}

	function handleWorkspacePanelGuestShortcuts(event: Event, input: Input) {
		if (handleDarwinCloseWindowShortcut(event, input, hideMainWindow, true)) return true;
		if (handleReloadShortcut(event, input)) return true;
		if (context.getShortcutCaptureActive()) return false;
		const shortcuts = getShortcuts();
		const forwardActions: ShortcutAction[] = [
			'toggleSidebar',
			'openWebSearch',
			'openGitPanel',
			'openWikiPanel',
			'openWorkspacePanel',
			'openFileSearch',
			'openTerminal',
			'toggleWorkspacePanel',
			'navigateBack',
			'navigateForward',
			'openSettings',
			'newChat',
			'findInSession',
			'focusSearch',
			'openJobs',
			'openSkillDrafts',
			'openGallery',
			'openInbox',
			'recentSession'
		];
		for (const action of forwardActions) {
			if (matchesInputShortcut(input, shortcuts[action])) {
				event.preventDefault();
				routeSignals.sendShortcutAction(action);
				return true;
			}
		}

		if (context.getSessionNavigationSuspended()) return false;
		if (matchesInputShortcut(input, shortcuts.previousSession)) {
			event.preventDefault();
			routeSignals.sendShortcutAction('previousSession');
			return true;
		}
		if (matchesInputShortcut(input, shortcuts.nextSession)) {
			event.preventDefault();
			routeSignals.sendShortcutAction('nextSession');
			return true;
		}
		return false;
	}

	function attachWebviewPanelShortcuts(webContents: WebContents) {
		webContents.on('before-input-event', (event, input) => {
			handleWorkspacePanelGuestShortcuts(event, input);
		});
	}

	function refreshGlobalShortcuts() {
		if (!app.isReady()) return;
		if (registeredMiniWindowShortcut) {
			globalShortcut.unregister(registeredMiniWindowShortcut);
			registeredMiniWindowShortcut = '';
		}
		const accelerator = shortcutBindingToAccelerator(getShortcuts().toggleMiniWindow);
		if (!accelerator) return;
		const registered = globalShortcut.register(accelerator, () => {
			if (context.getShortcutCaptureActive()) return;
			void toggleMiniWindow();
		});
		if (!registered) {
			console.warn(`Failed to register mini window shortcut: ${accelerator}`);
			return;
		}
		registeredMiniWindowShortcut = accelerator;
	}

	return {
		attachMainWindowShortcuts,
		attachMiniWindowShortcuts,
		attachSettingsWindowShortcuts,
		attachWebviewPanelShortcuts,
		refreshGlobalShortcuts,
		unregisterAll: () => globalShortcut.unregisterAll()
	};
}
