// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { makePasteChip } from './rich-composer-chips';
import {
	selectionTextWithPasteChips,
	serialize,
	serializePasteDisplay
} from './rich-composer-decorate';
import {
	PASTE_COLLAPSE_MIN_CHARS,
	parsePastedDisplay,
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

describe('parsePastedDisplay', () => {
	it('keeps typed text outside the paste token', () => {
		expect(parsePastedDisplay('[Pasted ~12805 lines] hihi')).toEqual([
			{ kind: 'paste', label: 'Pasted ~12805 lines' },
			{ kind: 'text', text: ' hihi' }
		]);
		expect(parsePastedDisplay('see [Pasted text] tail')).toEqual([
			{ kind: 'text', text: 'see ' },
			{ kind: 'paste', label: 'Pasted text' },
			{ kind: 'text', text: ' tail' }
		]);
	});

	it('does not invent a boundary inside a raw paste', () => {
		expect(parsePastedDisplay('line one\nline two\nhihi')).toBeNull();
	});
});

describe('serializePasteDisplay', () => {
	it('projects a chip plus typed text without the paste body', () => {
		const editor = document.createElement('div');
		const body = 'alpha\nbeta\ngamma';
		editor.append(makePasteChip(body), document.createTextNode('\u00a0hihi'));
		expect(serializePasteDisplay(editor)).toBe('[Pasted ~3 lines] hihi');
		expect(serialize(editor)).toBe(`${body}\u00a0hihi`);
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
