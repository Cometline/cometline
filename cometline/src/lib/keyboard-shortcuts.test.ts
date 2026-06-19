import { describe, expect, it } from 'vitest';
import { captureShortcut, matchesShortcut, normalizeKeyboardShortcuts } from './keyboard-shortcuts';

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
		const binding = captureShortcut(keyEvent({ key: 'ArrowUp', metaKey: true, altKey: true }));
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

	it('distinguishes send message from insert newline on Enter', () => {
		const send = { key: 'Enter', shift: false };
		const newline = { key: 'Enter', shift: true };
		expect(matchesShortcut(keyEvent({ key: 'Enter' }), send)).toBe(true);
		expect(matchesShortcut(keyEvent({ key: 'Enter', shiftKey: true }), send)).toBe(false);
		expect(matchesShortcut(keyEvent({ key: 'Enter', shiftKey: true }), newline)).toBe(true);
		expect(matchesShortcut(keyEvent({ key: 'Enter' }), newline)).toBe(false);
	});

	it('migrates legacy bare Enter send bindings', () => {
		const normalized = normalizeKeyboardShortcuts({
			sendMessage: { key: 'Enter' }
		});
		expect(normalized.sendMessage).toEqual({ key: 'Enter', shift: false });
		expect(normalized.insertNewline).toEqual({ key: 'Enter', shift: true });
		expect(
			matchesShortcut(keyEvent({ key: 'Enter', shiftKey: true }), normalized.sendMessage)
		).toBe(false);
	});

	it('captureShortcut records shift false for plain Enter', () => {
		expect(captureShortcut(keyEvent({ key: 'Enter' }))).toEqual({
			key: 'Enter',
			shift: false
		});
		expect(captureShortcut(keyEvent({ key: 'Enter', shiftKey: true }))).toEqual({
			key: 'Enter',
			shift: true
		});
	});

	it('includes openWebPanel default shortcut', () => {
		const normalized = normalizeKeyboardShortcuts({});
		expect(normalized.openWebPanel).toEqual({ command: true, key: 'o' });
	});
});
