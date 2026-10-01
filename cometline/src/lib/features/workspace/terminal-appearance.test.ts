import { describe, expect, it } from 'vitest';
import {
	DEFAULT_TERMINAL_APPEARANCE,
	normalizeTerminalAppearance,
	normalizeTerminalFontSize
} from './terminal-appearance';

describe('terminal appearance', () => {
	it('defaults missing settings for existing users', () => {
		expect(normalizeTerminalAppearance(undefined)).toEqual(DEFAULT_TERMINAL_APPEARANCE);
	});

	it('clamps text size and ignores retired font-family settings', () => {
		expect(
			normalizeTerminalAppearance({
				fontSize: 100,
				theme: 'unknown' as never
			})
		).toEqual({ ...DEFAULT_TERMINAL_APPEARANCE, fontSize: 32 });
	});

	it('preserves valid terminal settings', () => {
		expect(
			normalizeTerminalAppearance({
				fontSize: 13.4,
				theme: 'dracula'
			})
		).toEqual({
			fontSize: 13,
			theme: 'dracula'
		});
	});

	it('accepts the Gruvbox Dark color scheme', () => {
		expect(normalizeTerminalAppearance({ theme: 'gruvbox-dark' }).theme).toBe('gruvbox-dark');
	});

	it('keeps interactive text-size input within the supported layout range', () => {
		expect(normalizeTerminalFontSize(1_000_000)).toBe(32);
		expect(normalizeTerminalFontSize(-1_000_000)).toBe(8);
		expect(normalizeTerminalFontSize(Number.NaN)).toBe(12);
	});
});
