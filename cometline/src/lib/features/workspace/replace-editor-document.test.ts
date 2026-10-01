// @vitest-environment jsdom
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { afterEach, describe, expect, it } from 'vitest';
import { replaceEditorDocument } from './replace-editor-document';

describe('replaceEditorDocument', () => {
	let view: EditorView | null = null;

	afterEach(() => {
		view?.destroy();
		view = null;
	});

	it('does nothing when the document is unchanged', () => {
		const parent = document.createElement('div');
		document.body.appendChild(parent);
		view = new EditorView({ state: EditorState.create({ doc: 'hello' }), parent });
		expect(replaceEditorDocument(view, 'hello')).toBe(false);
		expect(view.state.doc.toString()).toBe('hello');
	});

	it('replaces the document when the text changed', () => {
		const parent = document.createElement('div');
		document.body.appendChild(parent);
		view = new EditorView({ state: EditorState.create({ doc: 'hello' }), parent });
		expect(replaceEditorDocument(view, 'hello world')).toBe(true);
		expect(view.state.doc.toString()).toBe('hello world');
	});
});
