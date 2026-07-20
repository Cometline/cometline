import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	shortcutTooltipKbd,
	shortcutTooltipText,
	resolveShortcutBinding
} from './shortcut-tooltip';
import type { KeyboardShortcuts } from '$lib/keyboard-shortcuts';

const macShortcuts: KeyboardShortcuts = {
	openTerminal: { command: true, key: 'j' },
	openWebPanel: { command: true, key: 'o' },
	sendMessage: { key: 'Enter' }
};

describe('shortcut-tooltip helpers', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('resolveShortcutBinding returns the live binding', () => {
		expect(resolveShortcutBinding('openTerminal', macShortcuts)).toEqual({
			command: true,
			key: 'j'
		});
		expect(resolveShortcutBinding(undefined, macShortcuts)).toBeUndefined();
		expect(resolveShortcutBinding('openJobs', macShortcuts)).toBeUndefined();
	});

	it('shortcutTooltipText appends a Mac-formatted chord when bound', () => {
		vi.stubGlobal('navigator', { platform: 'MacIntel' });
		expect(shortcutTooltipText('Open terminal', 'openTerminal', macShortcuts)).toBe(
			'Open terminal (⌘ j)'
		);
	});

	it('shortcutTooltipText uses Ctrl wording on non-Mac', () => {
		vi.stubGlobal('navigator', { platform: 'Win32' });
		expect(shortcutTooltipText('Open terminal', 'openTerminal', macShortcuts)).toBe(
			'Open terminal (Ctrl + j)'
		);
	});

	it('shortcutTooltipText returns label only when unbound or action omitted', () => {
		vi.stubGlobal('navigator', { platform: 'MacIntel' });
		expect(shortcutTooltipText('Jobs', 'openJobs', macShortcuts)).toBe('Jobs');
		expect(shortcutTooltipText('Copy', undefined, macShortcuts)).toBe('Copy');
	});

	it('shortcutTooltipText reflects a remapped binding', () => {
		vi.stubGlobal('navigator', { platform: 'MacIntel' });
		const remapped: KeyboardShortcuts = {
			...macShortcuts,
			openTerminal: { command: true, shift: true, key: 't' }
		};
		expect(shortcutTooltipText('Open terminal', 'openTerminal', remapped)).toBe(
			'Open terminal (⌘ ⇧ t)'
		);
	});

	it('shortcutTooltipKbd returns only the chord', () => {
		vi.stubGlobal('navigator', { platform: 'MacIntel' });
		expect(shortcutTooltipKbd('openTerminal', macShortcuts)).toBe('⌘ j');
		expect(shortcutTooltipKbd('openJobs', macShortcuts)).toBeUndefined();
		expect(shortcutTooltipKbd(undefined, macShortcuts)).toBeUndefined();
	});
});
