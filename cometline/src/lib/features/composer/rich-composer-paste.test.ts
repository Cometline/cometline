// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { makePasteChip } from './rich-composer-chips';
import { selectionTextWithPasteChips, serialize } from './rich-composer-decorate';
import {
	PASTE_COLLAPSE_MIN_CHARS,
	PASTE_DOM_MAX_CHARS,
	canExpandPastedText,
	exceedsComposerDomBudget,
	pastedTextLabel,
	shouldCollapsePastedText,
	storedPastedText
} from './rich-composer-paste';

describe('shouldCollapsePastedText', () => {
	it('leaves short single-line pastes editable', () => {
		expect(shouldCollapsePastedText('hello')).toBe(false);
		expect(shouldCollapsePastedText('a\nb')).toBe(false);
		expect(shouldCollapsePastedText('   \n  ')).toBe(false);
		expect(shouldCollapsePastedText('x'.repeat(PASTE_COLLAPSE_MIN_CHARS))).toBe(false);
	});

	it('collapses OpenCode-sized pastes', () => {
		expect(shouldCollapsePastedText('a\nb\nc')).toBe(true);
		expect(shouldCollapsePastedText(`  \r\na\r\nb\nc  `)).toBe(true);
		expect(shouldCollapsePastedText('x'.repeat(PASTE_COLLAPSE_MIN_CHARS + 1))).toBe(true);
	});
});

describe('storedPastedText', () => {
	it('trims clipboard padding and normalizes newlines', () => {
		expect(storedPastedText('  a\r\nb\r\nc  ')).toBe('a\nb\nc');
		expect(storedPastedText('short')).toBeNull();
	});
});

describe('pasted text chip', () => {
	it('shows a short label and serializes the stored text', () => {
		const text = 'alpha\nbeta\ngamma';
		const chip = makePasteChip(text);
		expect(chip.textContent).toBe(pastedTextLabel(text));
		expect(chip.textContent).not.toContain('alpha');
		expect(chip.dataset.pastedText).toBe(text);

		const editor = document.createElement('div');
		editor.append('see ', chip, ' tail');
		expect(serialize(editor)).toBe(`see ${text} tail`);
	});

	it('keeps crash-sized text out of the chip label', () => {
		const huge = 'x'.repeat(PASTE_DOM_MAX_CHARS + 1);
		expect(exceedsComposerDomBudget(huge)).toBe(true);
		expect(canExpandPastedText(huge)).toBe(false);
		const chip = makePasteChip(huge);
		expect(chip.textContent).toBe('Pasted text');
		expect(chip.textContent).not.toContain(huge.slice(0, 20));
		expect(chip.title.length).toBeLessThan(200);
	});

	it('copies the stored text instead of the chip label', () => {
		const editor = document.createElement('div');
		document.body.append(editor);
		const text = 'one\ntwo\nthree';
		editor.append('before ', makePasteChip(text));

		const range = document.createRange();
		range.selectNodeContents(editor);
		const sel = window.getSelection();
		sel?.removeAllRanges();
		sel?.addRange(range);

		expect(selectionTextWithPasteChips(editor)).toBe(`before ${text}`);
		editor.remove();
	});
});
