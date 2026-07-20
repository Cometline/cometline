import {
	formatShortcut,
	type KeyboardShortcuts,
	type ShortcutAction,
	type ShortcutBinding
} from '$lib/keyboard-shortcuts';

/** Resolve the live binding for a shortcut action, if any. */
export function resolveShortcutBinding(
	action: ShortcutAction | undefined,
	shortcuts: KeyboardShortcuts
): ShortcutBinding | undefined {
	if (!action) return undefined;
	return shortcuts[action];
}

/**
 * Accessible / visual tip copy: action label plus formatted shortcut when bound.
 * Returns only the label when there is no action or no binding.
 */
export function shortcutTooltipText(
	label: string,
	action: ShortcutAction | undefined,
	shortcuts: KeyboardShortcuts
): string {
	const binding = resolveShortcutBinding(action, shortcuts);
	if (!binding) return label;
	const formatted = formatShortcut(binding);
	if (!formatted || formatted === 'None') return label;
	return `${label} (${formatted})`;
}

/** Formatted shortcut chord for kbd display, or undefined when unbound. */
export function shortcutTooltipKbd(
	action: ShortcutAction | undefined,
	shortcuts: KeyboardShortcuts
): string | undefined {
	const binding = resolveShortcutBinding(action, shortcuts);
	if (!binding) return undefined;
	const formatted = formatShortcut(binding);
	if (!formatted || formatted === 'None') return undefined;
	return formatted;
}
