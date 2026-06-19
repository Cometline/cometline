/**
 * CometMind appends attached workspace files to the persisted user message as
 * `[File: path]` fenced blocks for the agent. Those blocks should not appear in
 * the chat UI — @ mentions are rendered as clickable chips instead.
 */

const FILE_HEADER = /^\[File: [^\]]+\]$/;
const OPEN_FENCE = /^(`{3,})(.*)$/;
const INCLUDE_ERROR = /^<!-- Could not include /;

function endOfFileBlock(lines: string[], headerIndex: number): number {
	const openLineIndex = headerIndex + 1;
	if (openLineIndex >= lines.length) return headerIndex + 1;

	const openMatch = lines[openLineIndex].match(OPEN_FENCE);
	if (!openMatch) return headerIndex + 1;

	const fence = openMatch[1];
	for (let i = lines.length - 1; i > openLineIndex; i--) {
		if (lines[i] === fence) return i + 1;
	}
	return headerIndex + 1;
}

export function stripInlinedFileBlocks(text: string): string {
	if (!text) return '';

	const lines = text.split('\n');
	const kept: string[] = [];
	let i = 0;

	while (i < lines.length) {
		const line = lines[i];

		if (INCLUDE_ERROR.test(line)) {
			i += 1;
			continue;
		}

		if (FILE_HEADER.test(line)) {
			i = endOfFileBlock(lines, i);
			continue;
		}

		kept.push(line);
		i += 1;
	}

	return kept.join('\n').replace(/\n{3,}/g, '\n\n').trimEnd();
}
