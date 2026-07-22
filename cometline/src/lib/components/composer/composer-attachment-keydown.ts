/** Decide which external composer attachment to remove when text is empty. */

export type AttachmentRemoval =
	| { kind: 'image'; id: string }
	| { kind: 'webContext'; index: number }
	| null;

/**
 * When the composer text is empty, prefer removing the last image, then the
 * last web-context chip. Returns null when there is nothing to remove.
 */
export function nextAttachmentRemoval(
	text: string,
	images: { id?: string }[],
	webContextCount: number
): AttachmentRemoval {
	if (text.trim() !== '') return null;

	for (let i = images.length - 1; i >= 0; i--) {
		const id = images[i]?.id;
		if (id) return { kind: 'image', id };
	}

	if (webContextCount > 0) {
		return { kind: 'webContext', index: webContextCount - 1 };
	}

	return null;
}
