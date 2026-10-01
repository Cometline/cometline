const mentionQueryChars = /^[a-zA-Z0-9_/.-]*$/;

export interface ActiveMention {
	query: string;
	range: Range;
}

export function findActiveMention(
	editor: HTMLElement | null,
	mentionsEnabled: boolean
): ActiveMention | null {
	if (!editor || !mentionsEnabled) return null;
	const sel = window.getSelection();
	if (!sel || sel.rangeCount === 0) return null;
	const focusNode = sel.focusNode;
	if (!focusNode || focusNode.nodeType !== Node.TEXT_NODE) return null;
	if (!editor.contains(focusNode)) return null;

	const text = focusNode.textContent ?? '';
	const offset = sel.focusOffset;

	let atIndex = -1;
	for (let i = offset - 1; i >= 0; i--) {
		const ch = text[i];
		if (ch === '@') {
			atIndex = i;
			break;
		}
		if (/\s/.test(ch)) break;
	}
	if (atIndex < 0) return null;

	// Require a word boundary before the '@' so email addresses don't trigger.
	if (atIndex > 0 && !/\s/.test(text[atIndex - 1])) return null;

	const query = text.slice(atIndex + 1, offset);
	if (!mentionQueryChars.test(query)) return null;

	const range = document.createRange();
	range.setStart(focusNode, atIndex);
	range.setEnd(focusNode, offset);
	return { query, range };
}

export function replaceRangeWithNodes(range: Range, nodes: Node[]) {
	range.deleteContents();
	const frag = document.createDocumentFragment();
	for (const node of nodes) {
		frag.appendChild(node);
	}
	range.insertNode(frag);
	range.collapse(false);
	const sel = window.getSelection();
	sel?.removeAllRanges();
	sel?.addRange(range);
}

export function setCaretPosition(editor: HTMLElement | null, atEnd: boolean) {
	if (!editor) return;
	const sel = window.getSelection();
	if (!sel) return;

	const range = document.createRange();
	const hasText = Boolean(editor.textContent);

	if (!hasText) {
		// Browsers often inject a lone <br> into empty contenteditables, which
		// makes a collapsed "end" caret render on a phantom second line.
		if (editor.innerHTML === '<br>') {
			editor.innerHTML = '';
		}
		range.setStart(editor, 0);
		range.collapse(true);
	} else if (atEnd) {
		range.selectNodeContents(editor);
		range.collapse(false);
	} else {
		const walker = document.createTreeWalker(editor, NodeFilter.SHOW_TEXT);
		const firstText = walker.nextNode();
		if (firstText) {
			range.setStart(firstText, 0);
			range.collapse(true);
		} else {
			range.setStart(editor, 0);
			range.collapse(true);
		}
	}

	sel.removeAllRanges();
	sel.addRange(range);
}
