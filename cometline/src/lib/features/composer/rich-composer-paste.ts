/** OpenCode collapses a paste at 3 lines or 150 characters. */
export const PASTE_COLLAPSE_MIN_LINES = 3;
export const PASTE_COLLAPSE_MIN_CHARS = 150;

export function pastedLineCount(text: string): number {
	if (!text) return 0;
	return (text.match(/\n/g)?.length ?? 0) + 1;
}

export function shouldCollapsePastedText(text: string): boolean {
	const trimmed = text.replace(/\r\n?/g, '\n').trim();
	if (!trimmed) return false;
	return (
		pastedLineCount(trimmed) >= PASTE_COLLAPSE_MIN_LINES ||
		trimmed.length > PASTE_COLLAPSE_MIN_CHARS
	);
}

/** Trimmed paste body, or null when the paste should stay editable text. */
export function storedPastedText(text: string): string | null {
	const trimmed = text.replace(/\r\n?/g, '\n').trim();
	return shouldCollapsePastedText(trimmed) ? trimmed : null;
}

export function pastedTextLabel(text: string): string {
	const lines = pastedLineCount(text);
	return lines >= 2 ? `Pasted ~${lines} lines` : 'Pasted text';
}

export function pastedDisplayToken(text: string): string {
	return `[${pastedTextLabel(text)}]`;
}

export type PastedDisplayPart =
	| { kind: 'text'; text: string }
	| { kind: 'paste'; label: string; text?: string };

const PASTE_DISPLAY_TOKEN = /\[(Pasted ~\d+ lines|Pasted text)\]/g;

/**
 * Put each stored paste body back into the matching `[Pasted …]` token.
 * Bodies are assigned in order. A leftover token stays a label when history
 * was saved before bodies were stored.
 */
export function restorePastedDisplay(
	display: string,
	pastes?: readonly string[]
): PastedDisplayPart[] | null {
	const parts = parsePastedDisplay(display);
	if (!parts || !pastes?.length) return parts;
	let bodyIndex = 0;
	return parts.map((part) => {
		if (part.kind !== 'paste') return part;
		const body = pastes[bodyIndex];
		bodyIndex += 1;
		return body ? { ...part, text: body } : part;
	});
}

/** Split a display document back into chips and typed text. Null if it is not one. */
export function parsePastedDisplay(source: string): PastedDisplayPart[] | null {
	if (!source.includes('[Pasted ')) return null;
	const parts: PastedDisplayPart[] = [];
	let cursor = 0;
	for (const match of source.matchAll(PASTE_DISPLAY_TOKEN)) {
		const start = match.index ?? 0;
		if (start > cursor) parts.push({ kind: 'text', text: source.slice(cursor, start) });
		parts.push({ kind: 'paste', label: match[1] ?? 'Pasted text' });
		cursor = start + match[0].length;
	}
	if (parts.length === 0) return null;
	if (cursor < source.length) parts.push({ kind: 'text', text: source.slice(cursor) });
	return parts;
}
