import { describe, expect, it } from 'vitest';
import {
	captureShortcut,
	matchesShortcut,
	normalizeKeyboardShortcuts
} from './keyboard-shortcuts';

function keyEvent(init: {
	key: string;
	ctrlKey?: boolean;
	metaKey?: boolean;
	altKey?: boolean;
	shiftKey?: boolean;
}): KeyboardEvent {
	return {
		key: init.key,
		ctrlKey: init.ctrlKey ?? false,
		metaKey: init.metaKey ?? false,
		altKey: init.altKey ?? false,
		shiftKey: init.shiftKey ?? false
	} as KeyboardEvent;
}

describe('keyboard-shortcuts', () => {
	it('captureShortcut preserves Option with Command on Mac', () => {
		const binding = captureShortcut(
			keyEvent({ key: 'ArrowUp', metaKey: true, altKey: true })
		);
		expect(binding).toEqual({ key: 'ArrowUp', alt: true, command: true });
	});

	it('keeps ⌘⌥ session navigation bindings when normalizing saved settings', () => {
		const normalized = normalizeKeyboardShortcuts({
			previousSession: { command: true, alt: true, key: 'ArrowUp' },
			nextSession: { command: true, alt: true, key: 'ArrowDown' }
		});
		expect(normalized.previousSession).toEqual({
			command: true,
			alt: true,
			key: 'ArrowUp'
		});
		expect(normalized.nextSession).toEqual({
			command: true,
			alt: true,
			key: 'ArrowDown'
		});
	});

	it('migrates legacy bare ⌘+arrow session navigation bindings', () => {
		const normalized = normalizeKeyboardShortcuts({
			previousSession: { command: true, key: 'ArrowUp' }
		});
		expect(normalized.previousSession).toEqual({
			ctrl: true,
			meta: true,
			key: 'ArrowUp'
		});
	});

	it('matches ⌘⌥ session navigation shortcuts', () => {
		const binding = { command: true, alt: true, key: 'ArrowUp' };
		expect(
			matchesShortcut(keyEvent({ key: 'ArrowUp', metaKey: true, altKey: true }), binding)
		).toBe(true);
		expect(
			matchesShortcut(keyEvent({ key: 'ArrowUp', metaKey: true, altKey: false }), binding)
		).toBe(false);
	});
});
