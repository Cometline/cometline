import { EditorSelection } from '@codemirror/state';
import type { EditorView } from '@codemirror/view';

/** Replace the document while keeping the current scroll position. */
export function replaceEditorDocument(view: EditorView, value: string): boolean {
	const current = view.state.doc.toString();
	if (current === value) return false;
	const { from, to } = view.state.selection.main;
	const nextLen = value.length;
	view.dispatch({
		changes: { from: 0, to: view.state.doc.length, insert: value },
		selection: EditorSelection.single(Math.min(from, nextLen), Math.min(to, nextLen)),
		effects: view.scrollSnapshot()
	});
	return true;
}
