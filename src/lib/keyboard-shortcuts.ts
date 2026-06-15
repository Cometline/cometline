export type ShortcutAction =
	| 'toggleSidebar'
	| 'openSettings'
	| 'newChat'
	| 'stopResponse'
	| 'sendMessage'
	| 'closeSettings'
	| 'focusSearch'
	| 'previousSession'
	| 'nextSession';

export interface ShortcutBinding {
	key: string;
	command?: boolean;
	ctrl?: boolean;
	meta?: boolean;
	alt?: boolean;
	shift?: boolean;
}

export interface KeyboardShortcutDefinition {
	id: ShortcutAction;
	label: string;
	defaultBinding: ShortcutBinding;
}

export type KeyboardShortcuts = Partial<Record<ShortcutAction, ShortcutBinding>>;

export const SHORTCUT_DEFINITIONS: KeyboardShortcutDefinition[] = [
	{
		id: 'toggleSidebar',
		label: 'Toggle sidebar',
		defaultBinding: { command: true, key: 'b' }
	},
	{
		id: 'openSettings',
		label: 'Open settings',
		defaultBinding: { command: true, key: ',' }
	},
	{
		id: 'newChat',
		label: 'New chat',
		defaultBinding: { command: true, key: 't' }
	},
	{
		id: 'stopResponse',
		label: 'Stop response',
		defaultBinding: { command: true, key: 'c' }
	},
	{
		id: 'sendMessage',
		label: 'Send message',
		defaultBinding: { key: 'Enter', shift: false }
	},
	{
		id: 'closeSettings',
		label: 'Close settings',
		defaultBinding: { key: 'Escape' }
	},
	{
		id: 'focusSearch',
		label: 'Focus search chats',
		defaultBinding: { command: true, key: 'f' }
	},
	{
		id: 'previousSession',
		label: 'Previous chat',
		defaultBinding: { command: true, key: 'ArrowUp' }
	},
	{
		id: 'nextSession',
		label: 'Next chat',
		defaultBinding: { command: true, key: 'ArrowDown' }
	}
];

const MODIFIER_KEYS = new Set(['Control', 'Shift', 'Alt', 'Meta']);

function keyMatches(a: string, b: string): boolean {
	return a === b || a.toLowerCase() === b.toLowerCase();
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
	if (!keyMatches(event.key, binding.key)) return false;

	const expectsCommand = binding.command ?? false;
	if (expectsCommand) {
		const hasCommand = event.ctrlKey || event.metaKey;
		if (!hasCommand) return false;
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

export function captureShortcut(event: KeyboardEvent): ShortcutBinding | null {
	if (MODIFIER_KEYS.has(event.key)) return null;

	const binding: ShortcutBinding = { key: event.key };
	const hasCtrl = event.ctrlKey;
	const hasMeta = event.metaKey;
	const hasAlt = event.altKey;
	const hasShift = event.shiftKey;

	// Store a lone Ctrl or Meta as the cross-platform "command" modifier so the
	// same binding renders and behaves as Cmd-or-Ctrl on macOS and Windows.
	const commandModifier = (hasCtrl || hasMeta) && !hasAlt;
	if (commandModifier && !(hasCtrl && hasMeta)) {
		binding.command = true;
	} else {
		if (hasCtrl) binding.ctrl = true;
		if (hasMeta) binding.meta = true;
		if (hasAlt) binding.alt = true;
		if (hasShift) binding.shift = true;
	}

	return binding;
}

export function formatShortcut(binding: ShortcutBinding | undefined): string {
	if (!binding) return 'None';
	const isMac = navigator.platform.toLowerCase().includes('mac');
	const parts: string[] = [];

	if (binding.command) {
		parts.push(isMac ? '⌘' : 'Ctrl');
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
