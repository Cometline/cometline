/** Pure DOM helpers for contenteditable caret / selection edge checks. */

function isEditorVisuallyEmpty(root: Node): boolean {
	const text = (root.textContent ?? '').replace(/\u00a0/g, '');
	return text.trim() === '';
}

function selectionInRoot(root: Node, sel: Selection): boolean {
	if (sel.rangeCount === 0) return false;
	const anchor = sel.anchorNode;
	if (!anchor) return false;
	return root === anchor || root.contains(anchor);
}

/**
 * Whether the collapsed caret sits at the start or end of `root`'s contents.
 * If there is no/invalid selection in `root`, an empty editor is treated as both edges.
 */
export function isSelectionAtEditorEdge(
	root: Node | null | undefined,
	edge: 'start' | 'end'
): boolean {
	if (!root) return true;

	const sel = typeof window !== 'undefined' ? window.getSelection() : null;
	if (!sel || !selectionInRoot(root, sel) || !sel.isCollapsed || sel.rangeCount === 0) {
		return isEditorVisuallyEmpty(root);
	}

	const caret = sel.getRangeAt(0);
	if (edge === 'start') {
		const before = document.createRange();
		before.selectNodeContents(root);
		before.setEnd(caret.startContainer, caret.startOffset);
		return before.toString() === '';
	}

	const after = document.createRange();
	after.selectNodeContents(root);
	after.setStart(caret.endContainer, caret.endOffset);
	return after.toString() === '';
}
