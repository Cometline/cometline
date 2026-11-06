import { shellStore } from '#lib/stores/shell.svelte.js';
import { gitFileName } from '#lib/features/workspace/git-changes-browser.js';
import { buildFileSnippetContext } from '#lib/features/workspace/selection-snippet.js';
import {
	firstSelectionClientRect,
	selectionPopupPosition
} from '#lib/features/workspace/selection-popup.js';

export type DiffSelectionPopup = {
	top: number;
	left: number;
	text: string;
};

const DIFF_CHAT_LIMIT = 50000;

export function addGitDiffToChat(filePath: string, diffText: string) {
	const content = diffText.trim().slice(0, DIFF_CHAT_LIMIT);
	if (!content) return;
	shellStore.addWebContextForActive({
		kind: 'file',
		title: `${gitFileName(filePath)} (diff)`,
		source: `workspace-file:${filePath}`,
		content
	});
	shellStore.requestComposerFocus();
}

/** Popup position for a selection inside `root`, or null when it should close. */
export function diffSelectionPopup(root: HTMLElement): DiffSelectionPopup | null {
	const sel = window.getSelection();
	if (!sel || sel.isCollapsed || sel.rangeCount === 0) return null;
	if (!root.contains(sel.anchorNode) || !root.contains(sel.focusNode)) return null;
	const text = sel.toString().trim();
	if (!text) return null;
	const rect = firstSelectionClientRect(sel.getRangeAt(0));
	return {
		...selectionPopupPosition(rect, window.innerWidth),
		text
	};
}

export function addDiffSelectionToChat(filePath: string, text: string) {
	const ctx = buildFileSnippetContext({ filePath, selectedText: text });
	if (!ctx) return;
	const title = text.includes('\n') ? `${gitFileName(filePath)} (diff selection)` : ctx.title;
	shellStore.addWebContextForActive({ ...ctx, title });
	shellStore.requestComposerFocus();
	window.getSelection()?.removeAllRanges();
}
