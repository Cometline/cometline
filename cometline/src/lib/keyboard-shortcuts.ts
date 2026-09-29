export type ShortcutAction =
	| 'toggleSidebar'
	| 'openSettings'
	| 'newChat'
	| 'toggleMiniWindow'
	| 'stopResponse'
	| 'sendMessage'
	| 'insertNewline'
	| 'closeSettings'
	| 'findInSession'
	| 'focusSearch'
	| 'previousSession'
	| 'nextSession'
	| 'toggleWorkspacePanel'
	| 'openWebSearch'
	| 'openGitPanel'
	| 'openWikiPanel'
	| 'openWorkspacePanel'
	| 'openFileSearch'
	| 'openTerminal'
	| 'navigateBack'
	| 'navigateForward'
	| 'openJobs'
	| 'openSkillDrafts'
	| 'openGallery'
	| 'openUsage'
	| 'openInbox'
	| 'cycleReasoningEffort'
	| 'recentSession';

export interface ShortcutBinding {
	key: string;
	command?: boolean;
	ctrl?: boolean;
	meta?: boolean;
	alt?: boolean;
	shift?: boolean;
}

export type ShortcutCategory = 'chats' | 'composer' | 'panels' | 'settings';

export interface ShortcutCategoryDefinition {
	id: ShortcutCategory;
	title: string;
	description: string;
}

export interface KeyboardShortcutDefinition {
	id: ShortcutAction;
	label: string;
	category: ShortcutCategory;
	defaultBinding: ShortcutBinding;
}

export type KeyboardShortcuts = Partial<Record<ShortcutAction, ShortcutBinding>>;

export const SHORTCUT_CATEGORIES: ShortcutCategoryDefinition[] = [
	{
		id: 'chats',
		title: 'Chats',
		description: 'Start and move between conversations.'
	},
	{
		id: 'composer',
		title: 'Composer',
		description: 'Send messages and control the active response.'
	},
	{
		id: 'panels',
		title: 'Panels',
		description: 'Show, hide, and open side panels.'
	},
	{
		id: 'settings',
		title: 'Settings',
		description: 'Open and dismiss the settings window.'
	}
];

export const SHORTCUT_DEFINITIONS: KeyboardShortcutDefinition[] = [
	{
		id: 'newChat',
		label: 'New chat',
		category: 'chats',
		defaultBinding: { command: true, key: 't' }
	},
	{
		id: 'toggleMiniWindow',
		label: 'Toggle mini window',
		category: 'chats',
		defaultBinding: { command: true, shift: true, key: 'l' }
	},
	{
		id: 'previousSession',
		label: 'Previous chat',
		category: 'chats',
		defaultBinding: { ctrl: true, meta: true, key: 'ArrowUp' }
	},
	{
		id: 'nextSession',
		label: 'Next chat',
		category: 'chats',
		defaultBinding: { ctrl: true, meta: true, key: 'ArrowDown' }
	},
	{
		id: 'recentSession',
		label: 'Return to recent chat',
		category: 'chats',
		defaultBinding: { command: true, shift: true, key: 'd' }
	},
	{
		id: 'findInSession',
		label: 'Find in current chat',
		category: 'chats',
		defaultBinding: { command: true, key: 'f' }
	},
	{
		id: 'focusSearch',
		label: 'Search chats',
		category: 'chats',
		defaultBinding: { command: true, shift: true, key: 'f' }
	},
	{
		id: 'sendMessage',
		label: 'Send message',
		category: 'composer',
		defaultBinding: { key: 'Enter', shift: false }
	},
	{
		id: 'insertNewline',
		label: 'Insert newline in composer',
		category: 'composer',
		defaultBinding: { key: 'Enter', shift: true }
	},
	{
		id: 'stopResponse',
		label: 'Stop response',
		category: 'composer',
		defaultBinding: { ctrl: true, meta: false, key: 'c' }
	},
	{
		id: 'cycleReasoningEffort',
		label: 'Cycle reasoning effort',
		category: 'composer',
		defaultBinding: { ctrl: true, meta: false, key: 't' }
	},
	{
		id: 'toggleSidebar',
		label: 'Toggle sidebar',
		category: 'panels',
		defaultBinding: { command: true, key: 'b' }
	},
	{
		id: 'toggleWorkspacePanel',
		label: 'Toggle workspace panel',
		category: 'panels',
		defaultBinding: { command: true, alt: true, key: 'b' }
	},
	{
		id: 'openWebSearch',
		label: 'New web tab',
		category: 'panels',
		defaultBinding: { command: true, key: 'o' }
	},
	{
		id: 'openGitPanel',
		label: 'Open git changes',
		category: 'panels',
		defaultBinding: { command: true, shift: true, key: 'g' }
	},
	{
		id: 'openWikiPanel',
		label: 'Open wiki files',
		category: 'panels',
		defaultBinding: { command: true, key: 'k' }
	},
	{
		id: 'openWorkspacePanel',
		label: 'Open workspace files',
		category: 'panels',
		defaultBinding: { command: true, key: 'l' }
	},
	{
		id: 'openFileSearch',
		label: 'Search files',
		category: 'panels',
		defaultBinding: { command: true, key: 'p' }
	},
	{
		id: 'openTerminal',
		label: 'Open terminal',
		category: 'panels',
		defaultBinding: { command: true, key: 'j' }
	},
	{
		id: 'navigateBack',
		label: 'Back (web page or recent chat)',
		category: 'panels',
		defaultBinding: { command: true, key: '[' }
	},
	{
		id: 'navigateForward',
		label: 'Forward (web page or recent chat)',
		category: 'panels',
		defaultBinding: { command: true, key: ']' }
	},
	{
		id: 'openJobs',
		label: 'Open jobs',
		category: 'panels',
		defaultBinding: { command: true, key: '1' }
	},
	{
		id: 'openSkillDrafts',
		label: 'Open skills',
		category: 'panels',
		defaultBinding: { command: true, key: '2' }
	},
	{
		id: 'openGallery',
		label: 'Open gallery',
		category: 'panels',
		defaultBinding: { command: true, key: '3' }
	},
	{
		id: 'openUsage',
		label: 'Open usage',
		category: 'panels',
		defaultBinding: { command: true, key: '4' }
	},
	{
		id: 'openInbox',
		label: 'Open inbox',
		category: 'panels',
		defaultBinding: { command: true, key: '5' }
	},
	{
		id: 'openSettings',
		label: 'Open settings',
		category: 'settings',
		defaultBinding: { command: true, key: ',' }
	},
	{
		id: 'closeSettings',
		label: 'Close settings',
		category: 'settings',
		defaultBinding: { key: 'Escape' }
	}
];

export function shortcutsByCategory(): Array<{
	category: ShortcutCategoryDefinition;
	shortcuts: KeyboardShortcutDefinition[];
}> {
	return SHORTCUT_CATEGORIES.map((category) => ({
		category,
		shortcuts: SHORTCUT_DEFINITIONS.filter((def) => def.category === category.id)
	}));
}

const MODIFIER_KEYS = new Set(['Control', 'Shift', 'Alt', 'Meta']);
const UNRELIABLE_KEY_VALUES = new Set(['', 'Process', 'Unidentified', 'Dead']);

const CODE_KEY_MAP: Record<string, string> = {
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

function keyMatches(a: string, b: string): boolean {
	return a === b || a.toLowerCase() === b.toLowerCase();
}

function hasShortcutModifier(event: KeyboardEvent): boolean {
	return event.ctrlKey || event.metaKey || event.altKey;
}

export function keyFromKeyboardCode(code: string | undefined): string | null {
	if (!code) return null;
	const letter = code.match(/^Key([A-Z])$/);
	if (letter) return letter[1].toLowerCase();
	const digit = code.match(/^Digit([0-9])$/);
	if (digit) return digit[1];
	const fn = code.match(/^F([1-9]|1[0-9]|2[0-4])$/);
	if (fn) return code.toUpperCase();
	return CODE_KEY_MAP[code] ?? null;
}

function isUnreliableKey(key: string): boolean {
	return UNRELIABLE_KEY_VALUES.has(key);
}

function shortcutEventKey(event: KeyboardEvent): string {
	const codeKey = keyFromKeyboardCode(event.code);
	if (
		codeKey &&
		(hasShortcutModifier(event) || isUnreliableKey(event.key) || event.isComposing)
	) {
		return codeKey;
	}
	return event.key;
}

export function defaultKeyboardShortcuts(): KeyboardShortcuts {
	return Object.fromEntries(
		SHORTCUT_DEFINITIONS.map((def) => [def.id, { ...def.defaultBinding }])
	) as KeyboardShortcuts;
}

export function normalizeKeyboardShortcuts(
	saved: KeyboardShortcuts | undefined
): KeyboardShortcuts {
	const defaults = defaultKeyboardShortcuts();
	if (!saved || typeof saved !== 'object') return defaults;

	const next: KeyboardShortcuts = { ...defaults };
	for (const def of SHORTCUT_DEFINITIONS) {
		const binding = saved[def.id];
		if (binding && typeof binding === 'object' && typeof binding.key === 'string') {
			next[def.id] = {
				key: binding.key,
				...(typeof binding.command === 'boolean' && { command: binding.command }),
				...(typeof binding.ctrl === 'boolean' && { ctrl: binding.ctrl }),
				...(typeof binding.meta === 'boolean' && { meta: binding.meta }),
				...(typeof binding.alt === 'boolean' && { alt: binding.alt }),
				...(typeof binding.shift === 'boolean' && { shift: binding.shift })
			};
		}
	}
	return next;
}

export function matchesShortcut(
	event: KeyboardEvent,
	binding: ShortcutBinding | undefined
): boolean {
	if (!binding) return false;
	if (!keyMatches(shortcutEventKey(event), binding.key)) return false;

	const expectsCommand = binding.command ?? false;
	if (expectsCommand) {
		const hasCommand = event.ctrlKey || event.metaKey;
		if (!hasCommand) return false;
		// ⌘-style bindings must not swallow ⌃⌘ chords (e.g. macOS ⌃⌘F fullscreen).
		if (binding.ctrl !== true && event.ctrlKey && event.metaKey) return false;
		if (binding.alt !== undefined ? binding.alt !== event.altKey : event.altKey) return false;
		if (binding.shift !== undefined ? binding.shift !== event.shiftKey : event.shiftKey)
			return false;
		return true;
	}

	if (binding.ctrl !== undefined && binding.ctrl !== event.ctrlKey) return false;
	if (binding.meta !== undefined && binding.meta !== event.metaKey) return false;
	if (binding.alt !== undefined && binding.alt !== event.altKey) return false;
	if (binding.shift !== undefined && binding.shift !== event.shiftKey) return false;
	return true;
}

/** Modifier/key snapshot shared by DOM KeyboardEvent and Electron Input. */
export type ReloadShortcutInput = {
	key?: string;
	code?: string;
	meta?: boolean;
	control?: boolean;
	alt?: boolean;
	shift?: boolean;
	isComposing?: boolean;
};

/**
 * Cmd+R (macOS) / Ctrl+R (elsewhere). Requires exactly one of meta/control.
 * Bare R must never match — used for app refresh confirm.
 */
export function isReloadShortcut(input: ReloadShortcutInput): boolean {
	if (input.isComposing) return false;
	if (input.alt === true || input.shift === true) return false;
	const hasMeta = input.meta === true;
	const hasControl = input.control === true;
	// Require a primary modifier first so a bare "r" can never match.
	if (hasMeta === hasControl) return false;
	const code = input.code ?? '';
	const key = String(input.key ?? '').toLowerCase();
	if (code !== 'KeyR' && key !== 'r') return false;
	return true;
}

export function captureShortcut(event: KeyboardEvent): ShortcutBinding | null {
	if (MODIFIER_KEYS.has(event.key)) return null;

	const hasCtrl = event.ctrlKey;
	const hasMeta = event.metaKey;
	const hasAlt = event.altKey;
	const hasShift = event.shiftKey;
	const capturedKey =
		hasCtrl || hasMeta || hasAlt ? (keyFromKeyboardCode(event.code) ?? event.key) : event.key;
	const binding: ShortcutBinding = { key: capturedKey };

	if (hasAlt) binding.alt = true;
	if (hasShift) {
		binding.shift = true;
	} else if (!hasCtrl && !hasMeta && !hasAlt && keyMatches(event.key, 'Enter')) {
		binding.shift = false;
	}

	// Lone Meta (Cmd on Mac) → cross-platform "command" modifier.
	if (hasMeta && !hasCtrl) {
		binding.command = true;
		return binding;
	}

	// Lone Ctrl → strict Control key (not Command on Mac).
	if (hasCtrl && !hasMeta) {
		binding.ctrl = true;
		binding.meta = false;
		return binding;
	}

	if (hasCtrl && hasMeta) {
		binding.ctrl = true;
		binding.meta = true;
	}

	return binding;
}

export function formatShortcut(binding: ShortcutBinding | undefined): string {
	if (!binding) return 'None';
	const isMac = navigator.platform.toLowerCase().includes('mac');
	const parts: string[] = [];

	if (binding.command) {
		parts.push(isMac ? '⌘' : 'Ctrl');
		if (binding.alt) parts.push(isMac ? '⌥' : 'Alt');
	} else {
		if (binding.ctrl) parts.push(isMac ? '⌃' : 'Ctrl');
		if (binding.meta) parts.push(isMac ? '⌘' : 'Win');
		if (binding.alt) parts.push(isMac ? '⌥' : 'Alt');
	}
	if (binding.shift) parts.push(isMac ? '⇧' : 'Shift');

	const key = binding.key === ' ' ? 'Space' : binding.key;
	parts.push(key);

	return parts.join(isMac ? ' ' : ' + ');
}

export function isDefaultBinding(
	action: ShortcutAction,
	binding: ShortcutBinding | undefined
): boolean {
	if (!binding) return false;
	const def = SHORTCUT_DEFINITIONS.find((d) => d.id === action);
	if (!def) return false;
	return (
		binding.key === def.defaultBinding.key &&
		binding.command === def.defaultBinding.command &&
		binding.ctrl === def.defaultBinding.ctrl &&
		binding.meta === def.defaultBinding.meta &&
		binding.alt === def.defaultBinding.alt &&
		binding.shift === def.defaultBinding.shift
	);
}
