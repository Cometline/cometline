import { isHttpUrl, fileMentionText } from '$lib/markdown/embed';
import { makeChip, makeSkillChip } from '$lib/features/composer/rich-composer-chips';

export type DecorateOptions = { allowCaretEnd?: boolean };

/**
 * Serializes the contenteditable DOM back to plain text. URL chips serialize
 * to their full URL (stored in data-url); <br>/block boundaries become \n.
 */
export function serialize(root: HTMLElement): string {
	let out = '';
	const walk = (node: Node) => {
		for (const child of Array.from(node.childNodes)) {
			if (child.nodeType === Node.TEXT_NODE) {
				out += child.textContent ?? '';
			} else if (child instanceof HTMLElement) {
				if (child.dataset.url) {
					out += child.dataset.url;
				} else if (child.dataset.skillCommand) {
					out += child.dataset.skillCommand;
				} else if (child.dataset.filePath) {
					out += fileMentionText(child.dataset.filePath);
				} else if (child.tagName === 'BR') {
					out += '\n';
				} else {
					const isBlock = /^(DIV|P)$/.test(child.tagName);
					if (isBlock && out && !out.endsWith('\n')) out += '\n';
					walk(child);
				}
			}
		}
	};
	walk(root);
	return out;
}

function escapeRegex(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function skillNameRegex(skillNames: string[]) {
	const names = skillNames.map((name) => name.trim()).filter(Boolean);
	if (names.length === 0) return null;
	const pattern = names
		.sort((a, b) => b.length - a.length)
		.map(escapeRegex)
		.join('|');
	return new RegExp(`(^|\\s)\\/(${pattern})(?=\\s|$)`, 'g');
}

/**
 * Scans text nodes in the editor and replaces any complete bare URL with a
 * chip. Only runs on text the user isn't actively typing the tail of (we
 * require a trailing boundary char or that the URL isn't at the caret end).
 */
export function linkifyEditor(editor: HTMLElement | null, opts?: DecorateOptions) {
	const allowCaretEnd = opts?.allowCaretEnd ?? false;
	if (!editor) return;
	const urlRe = /https?:\/\/[^\s<]+/g;
	const walker = document.createTreeWalker(editor, NodeFilter.SHOW_TEXT);
	const textNodes: Text[] = [];
	let n = walker.nextNode();
	while (n) {
		// Skip text already inside a chip.
		if (!(n.parentElement && n.parentElement.closest('.rce-chip'))) {
			textNodes.push(n as Text);
		}
		n = walker.nextNode();
	}

	const sel = window.getSelection();
	const caretNode = sel && sel.rangeCount > 0 ? sel.focusNode : null;
	const caretOffset = sel && sel.rangeCount > 0 ? sel.focusOffset : 0;

	let didChange = false;
	for (const textNode of textNodes) {
		const text = textNode.textContent ?? '';
		urlRe.lastIndex = 0;
		let match: RegExpExecArray | null = urlRe.exec(text);
		if (!match) continue;

		// Build replacement fragment.
		const frag = document.createDocumentFragment();
		let cursor = 0;
		let replaced = false;
		urlRe.lastIndex = 0;
		while ((match = urlRe.exec(text)) !== null) {
			const start = match.index;
			const end = start + match[0].length;
			// Don't chipify a URL the caret is still typing at the end of —
			// unless this is a paste, where we chipify immediately.
			const caretInThisNode = caretNode === textNode;
			const caretAtUrlEnd = caretInThisNode && caretOffset === end;
			if (caretAtUrlEnd && !allowCaretEnd) continue;
			let url = match[0];
			const trailing = /[.,;:!?)\]}'"]+$/.exec(url);
			if (trailing) url = url.slice(0, url.length - trailing[0].length);
			if (!isHttpUrl(url)) continue;

			if (start > cursor)
				frag.appendChild(document.createTextNode(text.slice(cursor, start)));
			frag.appendChild(makeChip(url));
			const suffix = match[0].slice(url.length);
			if (suffix) frag.appendChild(document.createTextNode(suffix));
			cursor = end;
			replaced = true;
		}
		if (!replaced) continue;
		if (cursor < text.length) frag.appendChild(document.createTextNode(text.slice(cursor)));

		// Append a trailing space + place caret after the inserted content so
		// typing continues normally after a chip.
		const trailingSpace = document.createTextNode('\u00a0');
		frag.appendChild(trailingSpace);
		textNode.replaceWith(frag);
		didChange = true;

		// Restore caret to just after the trailing space.
		const range = document.createRange();
		range.setStartAfter(trailingSpace);
		range.collapse(true);
		sel?.removeAllRanges();
		sel?.addRange(range);
	}
	return didChange;
}

export function skillifyEditor(
	editor: HTMLElement | null,
	skillNames: string[],
	opts?: DecorateOptions
) {
	const re = skillNameRegex(skillNames);
	if (!editor || !re) return;
	const allowCaretEnd = opts?.allowCaretEnd ?? false;
	const walker = document.createTreeWalker(editor, NodeFilter.SHOW_TEXT);
	const textNodes: Text[] = [];
	let n = walker.nextNode();
	while (n) {
		if (!(n.parentElement && n.parentElement.closest('.rce-chip'))) {
			textNodes.push(n as Text);
		}
		n = walker.nextNode();
	}

	const sel = window.getSelection();
	const caretNode = sel && sel.rangeCount > 0 ? sel.focusNode : null;
	const caretOffset = sel && sel.rangeCount > 0 ? sel.focusOffset : 0;

	let didChange = false;
	for (const textNode of textNodes) {
		const text = textNode.textContent ?? '';
		re.lastIndex = 0;
		if (!re.test(text)) continue;

		const frag = document.createDocumentFragment();
		let cursor = 0;
		let replaced = false;
		re.lastIndex = 0;
		let match: RegExpExecArray | null;
		while ((match = re.exec(text)) !== null) {
			const prefix = match[1] ?? '';
			const name = match[2];
			const commandStart = match.index + prefix.length;
			const commandEnd = commandStart + name.length + 1;
			const caretInThisNode = caretNode === textNode;
			const caretAtCommandEnd = caretInThisNode && caretOffset === commandEnd;
			const hasTrailingBoundary = commandEnd < text.length && /\s/.test(text[commandEnd]);
			if (caretAtCommandEnd && !allowCaretEnd && !hasTrailingBoundary) continue;

			if (commandStart > cursor) {
				frag.appendChild(document.createTextNode(text.slice(cursor, commandStart)));
			}
			frag.appendChild(makeSkillChip(name));
			cursor = commandEnd;
			replaced = true;
		}
		if (!replaced) continue;
		if (cursor < text.length) frag.appendChild(document.createTextNode(text.slice(cursor)));

		textNode.replaceWith(frag);
		didChange = true;
	}
	if (didChange) {
		const range = document.createRange();
		range.selectNodeContents(editor);
		range.collapse(false);
		sel?.removeAllRanges();
		sel?.addRange(range);
	}
	return didChange;
}

export function decorateEditor(
	editor: HTMLElement | null,
	skillNames: string[],
	opts?: DecorateOptions
) {
	const didSkillify = skillifyEditor(editor, skillNames, opts);
	const didLinkify = linkifyEditor(editor, opts);
	return didSkillify || didLinkify;
}
