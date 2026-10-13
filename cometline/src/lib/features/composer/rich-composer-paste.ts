/**
 * Paste collapse, matching OpenCode's prompt (`pasteInputText`):
 * 3+ lines or more than 150 characters become a placeholder instead of raw text.
 * A second, higher budget keeps crash-sized text out of the contenteditable entirely.
 */
export const PASTE_COLLAPSE_MIN_LINES = 3;
export const PASTE_COLLAPSE_MIN_CHARS = 150;
export const PASTE_DOM_MAX_CHARS = 64_000;
export const PASTE_DOM_MAX_LINES = 1_500;

export function normalizePastedText(text: string): string {
	return text.replace(/\r\n?/g, '\n');
}

export function pastedLineCount(text: string): number {
	if (!text) return 0;
	return (text.match(/\n/g)?.length ?? 0) + 1;
}

/** True when a paste should become a chip instead of editable text. */
export function shouldCollapsePastedText(text: string): boolean {
	const trimmed = normalizePastedText(text).trim();
	if (!trimmed) return false;
	return (
		pastedLineCount(trimmed) >= PASTE_COLLAPSE_MIN_LINES ||
		trimmed.length > PASTE_COLLAPSE_MIN_CHARS
	);
}

/** Text stored on the chip. OpenCode keeps the trimmed paste, not the clipboard padding. */
export function storedPastedText(text: string): string | null {
	const trimmed = normalizePastedText(text).trim();
	if (!shouldCollapsePastedText(trimmed)) return null;
	return trimmed;
}

/** True when putting this string in the contenteditable can freeze or crash the shell. */
export function exceedsComposerDomBudget(text: string): boolean {
	return text.length > PASTE_DOM_MAX_CHARS || pastedLineCount(text) > PASTE_DOM_MAX_LINES;
}

export function canExpandPastedText(text: string): boolean {
	return !exceedsComposerDomBudget(text);
}

export function pastedTextLabel(text: string): string {
	const lines = pastedLineCount(text);
	if (lines >= 2) return `Pasted ~${lines} lines`;
	return 'Pasted text';
}

export function pastedTextTitle(text: string): string {
	const flat = text.replace(/\s+/g, ' ').trim();
	const preview = flat.length > 160 ? `${flat.slice(0, 160)}…` : flat;
	return preview || 'Included when you send';
}
