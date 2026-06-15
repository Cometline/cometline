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
		defaultBinding: { ctrl: true, meta: true, key: 'ArrowUp' }
	},
	{
		id: 'nextSession',
		label: 'Next chat',
		defaultBinding: { ctrl: true, meta: true, key: 'ArrowDown' }
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

const SESSION_NAV_ACTIONS = new Set<ShortcutAction>(['previousSession', 'nextSession']);

function isLegacySessionNavBinding(binding: ShortcutBinding): boolean {
	if (binding.command) return true;
	return Boolean(binding.ctrl && binding.meta === false);
}

function normalizeSessionNavBinding(
	action: ShortcutAction,
	binding: ShortcutBinding | undefined,
	defaultBinding: ShortcutBinding
): ShortcutBinding {
	if (!SESSION_NAV_ACTIONS.has(action)) {
		return binding ?? defaultBinding;
	}
	if (!binding) return { ...defaultBinding };
	if (isLegacySessionNavBinding(binding)) {
		return { ...defaultBinding };
	}
	return binding;
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
			const normalized: ShortcutBinding = {
				key: binding.key,
				...(typeof binding.command === 'boolean' && { command: binding.command }),
				...(typeof binding.ctrl === 'boolean' && { ctrl: binding.ctrl }),
				...(typeof binding.meta === 'boolean' && { meta: binding.meta }),
				...(typeof binding.alt === 'boolean' && { alt: binding.alt }),
				...(typeof binding.shift === 'boolean' && { shift: binding.shift })
			};
			next[def.id] = normalizeSessionNavBinding(def.id, normalized, def.defaultBinding);
		} else {
			next[def.id] = normalizeSessionNavBinding(def.id, undefined, def.defaultBinding);
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

	if (hasAlt) binding.alt = true;
	if (hasShift) binding.shift = true;

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
