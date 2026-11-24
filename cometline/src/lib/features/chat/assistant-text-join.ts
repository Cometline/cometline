/** Paragraph break inserted between assistant beats in one bubble. */
const PARAGRAPH_BREAK = '\n\n';

/**
 * Join the next assistant beat onto text already in the bubble.
 * A single trailing newline is not a paragraph, so normalize to a blank line.
 * Streaming deltas and reloaded steps share this so the gap survives a refresh.
 */
export function joinAssistantText(current: string, next: string): string {
	if (!next) return current;
	if (!current.trim() || /^\s/.test(next)) return current + next;
	const left = current.replace(/\s+$/, '');
	return left + PARAGRAPH_BREAK + next;
}
