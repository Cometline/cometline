// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { isSelectionAtEditorEdge } from './composer-caret';

function placeCaret(root: HTMLElement, offset: number) {
	const text = root.firstChild;
	if (!text || text.nodeType !== Node.TEXT_NODE) {
		throw new Error('expected a text node');
	}
	const range = document.createRange();
	range.setStart(text, offset);
	range.collapse(true);
	const sel = window.getSelection();
	if (!sel) throw new Error('no selection');
	sel.removeAllRanges();
	sel.addRange(range);
}

describe('isSelectionAtEditorEdge', () => {
	it('treats null root and empty editor without selection as both edges', () => {
		expect(isSelectionAtEditorEdge(null, 'start')).toBe(true);
		expect(isSelectionAtEditorEdge(null, 'end')).toBe(true);

		const empty = document.createElement('div');
		document.body.appendChild(empty);
		window.getSelection()?.removeAllRanges();
		expect(isSelectionAtEditorEdge(empty, 'start')).toBe(true);
		expect(isSelectionAtEditorEdge(empty, 'end')).toBe(true);
		empty.remove();
	});

	it('detects start and end of a text node', () => {
		const root = document.createElement('div');
		root.textContent = 'hello';
		document.body.appendChild(root);

		placeCaret(root, 0);
		expect(isSelectionAtEditorEdge(root, 'start')).toBe(true);
		expect(isSelectionAtEditorEdge(root, 'end')).toBe(false);

		placeCaret(root, 5);
		expect(isSelectionAtEditorEdge(root, 'start')).toBe(false);
		expect(isSelectionAtEditorEdge(root, 'end')).toBe(true);

		placeCaret(root, 2);
		expect(isSelectionAtEditorEdge(root, 'start')).toBe(false);
		expect(isSelectionAtEditorEdge(root, 'end')).toBe(false);

		root.remove();
	});

	it('returns false for non-empty editor when selection is outside root', () => {
		const root = document.createElement('div');
		root.textContent = 'hello';
		const other = document.createElement('div');
		other.textContent = 'x';
		document.body.append(root, other);
		placeCaret(other, 0);
		expect(isSelectionAtEditorEdge(root, 'start')).toBe(false);
		expect(isSelectionAtEditorEdge(root, 'end')).toBe(false);
		root.remove();
		other.remove();
	});
});
