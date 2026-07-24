import type { DiffLineKind } from '$lib/tools/diff-artifact';
import { classifyDiffLine } from '$lib/tools/parse-edit-diff';

export type GitDiffLine = {
	kind: DiffLineKind;
	text: string;
};

/** Split a unified diff into classified lines for colorized rendering. */
export function parseGitDiffLines(diff: string): GitDiffLine[] {
	const text = String(diff ?? '').replace(/\r\n/g, '\n');
	if (!text) return [];
	// Keep trailing empty line if present so selection matches source.
	const raw = text.endsWith('\n') ? text.slice(0, -1).split('\n') : text.split('\n');
	return raw.map((line) => ({
		kind: classifyDiffLine(line),
		text: line
	}));
}
