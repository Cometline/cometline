import { describe, expect, it } from 'vitest';
import { captureShortcut, matchesShortcut, normalizeKeyboardShortcuts } from './keyboard-shortcuts';

function keyEvent(init: {
	key: string;
	code?: string;
	ctrlKey?: boolean;
	metaKey?: boolean;
	altKey?: boolean;
	shiftKey?: boolean;
	isComposing?: boolean;
}): KeyboardEvent {
	return {
		key: init.key,
		code: init.code ?? '',
		ctrlKey: init.ctrlKey ?? false,
		metaKey: init.metaKey ?? false,
		altKey: init.altKey ?? false,
		shiftKey: init.shiftKey ?? false,
		isComposing: init.isComposing ?? false
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

	it('matches modifier shortcuts by physical code for IME sentinel keys', () => {
		expect(
			matchesShortcut(keyEvent({ key: 'Process', code: 'KeyT', metaKey: true }), {
				command: true,
				key: 't'
			})
		).toBe(true);
		expect(
			matchesShortcut(
				keyEvent({ key: 'Unidentified', code: 'KeyB', metaKey: true, altKey: true }),
				{ command: true, alt: true, key: 'b' }
			)
		).toBe(true);
	});

	it('matches macOS Option-produced characters by physical code', () => {
		expect(
			matchesShortcut(keyEvent({ key: '∫', code: 'KeyB', metaKey: true, altKey: true }), {
				command: true,
				alt: true,
				key: 'b'
			})
		).toBe(true);
	});

	it('does not treat Ctrl+Cmd as a lone command chord (macOS fullscreen, etc.)', () => {
		const focusSearch = { command: true, key: 'f' };
		expect(matchesShortcut(keyEvent({ key: 'f', code: 'KeyF', metaKey: true }), focusSearch)).toBe(
			true
		);
		expect(
			matchesShortcut(
				keyEvent({ key: 'f', code: 'KeyF', metaKey: true, ctrlKey: true }),
				focusSearch
			)
		).toBe(false);
	});

	it('matches session navigation by physical arrow code under IME', () => {
		expect(
			matchesShortcut(
				keyEvent({ key: 'Process', code: 'ArrowUp', ctrlKey: true, metaKey: true }),
				{
					ctrl: true,
					meta: true,
					key: 'ArrowUp'
				}
			)
		).toBe(true);
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

	it('captureShortcut records physical key for modified Option characters', () => {
		expect(
			captureShortcut(keyEvent({ key: '∫', code: 'KeyB', metaKey: true, altKey: true }))
		).toEqual({
			key: 'b',
			alt: true,
			command: true
		});
	});

	it('includes openWebPanel default shortcut', () => {
		const normalized = normalizeKeyboardShortcuts({});
		expect(normalized.openWebPanel).toEqual({ command: true, key: 'o' });
	});

	it('includes openTerminal default shortcut', () => {
		const normalized = normalizeKeyboardShortcuts({});
		expect(normalized.openTerminal).toEqual({ command: true, key: 'j' });
	});

	it('includes shared navigate back/forward defaults', () => {
		const normalized = normalizeKeyboardShortcuts({});
		expect(normalized.navigateBack).toEqual({ command: true, key: '[' });
		expect(normalized.navigateForward).toEqual({ command: true, key: ']' });
	});

	it('includes jobs, skill drafts, and inbox panel shortcuts', () => {
		const normalized = normalizeKeyboardShortcuts({});
		expect(normalized.openJobs).toEqual({ command: true, key: '1' });
		expect(normalized.openSkillDrafts).toEqual({ command: true, key: '2' });
		expect(normalized.openInbox).toEqual({ command: true, key: '3' });
	});

	it('includes toggleMiniWindow default shortcut', () => {
		const normalized = normalizeKeyboardShortcuts({});
		expect(normalized.toggleMiniWindow).toEqual({ command: true, shift: true, key: 'l' });
	});

	it('defaults stop response to Ctrl+C while preserving a saved Cmd+C binding', () => {
		expect(normalizeKeyboardShortcuts({}).stopResponse).toEqual({
			ctrl: true,
			meta: false,
			key: 'c'
		});
		expect(
			normalizeKeyboardShortcuts({ stopResponse: { command: true, key: 'c' } }).stopResponse
		).toEqual({
			command: true,
			key: 'c'
		});
	});

	it('migrates macOS Option-produced toggle web panel binding', () => {
		const normalized = normalizeKeyboardShortcuts({
			toggleWebPanel: { command: true, alt: true, key: '∫' }
		});
		expect(normalized.toggleWebPanel).toEqual({ command: true, alt: true, key: 'b' });
	});
});
