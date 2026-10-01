function linesFromText(text: string): string[] {
	const normalized = String(text ?? '').replace(/\r\n/g, '\n');
	if (!normalized) return [];
	return normalized.endsWith('\n') ? normalized.slice(0, -1).split('\n') : normalized.split('\n');
}

function formatRange(start: number, count: number): string {
	return count === 1 ? String(start) : `${start},${count}`;
}

/**
 * Builds one context hunk around the changed range. This is intentionally
 * linear so comparing a large dirty draft stays responsive.
 */
export function createFileDiff(before: string, after: string): string {
	const oldLines = linesFromText(before);
	const newLines = linesFromText(after);
	let prefix = 0;
	while (
		prefix < oldLines.length &&
		prefix < newLines.length &&
		oldLines[prefix] === newLines[prefix]
	) {
		prefix += 1;
	}
	if (prefix === oldLines.length && prefix === newLines.length) return '';

	let suffix = 0;
	while (
		suffix < oldLines.length - prefix &&
		suffix < newLines.length - prefix &&
		oldLines[oldLines.length - suffix - 1] === newLines[newLines.length - suffix - 1]
	) {
		suffix += 1;
	}

	const contextStart = Math.max(0, prefix - 3);
	const oldChangedEnd = oldLines.length - suffix;
	const newChangedEnd = newLines.length - suffix;
	const oldEnd = Math.min(oldLines.length, oldChangedEnd + 3);
	const newEnd = Math.min(newLines.length, newChangedEnd + 3);
	const diff = ['--- Current draft', '+++ Disk version'];
	diff.push(
		`@@ -${formatRange(contextStart + 1, oldEnd - contextStart)} +${formatRange(
			contextStart + 1,
			newEnd - contextStart
		)} @@`
	);
	for (let i = contextStart; i < prefix; i += 1) diff.push(` ${oldLines[i]}`);
	for (let i = prefix; i < oldChangedEnd; i += 1) diff.push(`-${oldLines[i]}`);
	for (let i = prefix; i < newChangedEnd; i += 1) diff.push(`+${newLines[i]}`);
	for (let i = oldChangedEnd; i < oldEnd; i += 1) diff.push(` ${oldLines[i]}`);
	return `${diff.join('\n')}\n`;
}
