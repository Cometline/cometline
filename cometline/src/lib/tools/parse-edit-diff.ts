import {
	DIFF_BEGIN_MARKER,
	DIFF_END_MARKER,
	looksLikeDiffArtifact,
	type DiffArtifact,
	type DiffLineKind,
	type ParsedDiffLine
} from './diff-artifact';

export type { DiffArtifact, DiffLineKind, ParsedDiffLine };
export { DIFF_BEGIN_MARKER, DIFF_END_MARKER, looksLikeDiffArtifact };

/** Extract DiffArtifact from edit_file tool output (Format projection). */
export function parseEditDiff(output: string): DiffArtifact | null {
	const text = String(output ?? '').replace(/\r\n/g, '\n');
	const begin = text.indexOf(DIFF_BEGIN_MARKER);
	if (begin < 0) return null;

	let end = text.indexOf(DIFF_END_MARKER, begin + DIFF_BEGIN_MARKER.length);
	// Tolerate truncated streams that still have a usable unified diff body.
	if (end < 0) end = text.length;

	const summary = text.slice(0, begin).trim();
	let body = text.slice(begin + DIFF_BEGIN_MARKER.length, end);
	if (body.startsWith('\n')) body = body.slice(1);
	if (body.endsWith('\n')) body = body.slice(0, -1);
	if (!body.trim()) return null;

	const lines: ParsedDiffLine[] = body.split('\n').map((line) => ({
		kind: classifyDiffLine(line),
		text: line
	}));

	const artifact: DiffArtifact = { summary, lines };
	const m = /^edited\s+(.+?)\s+\((\+\d+)\s+(-\d+)\)/.exec(summary);
	if (m) {
		artifact.path = m[1];
		artifact.added = Number(m[2]);
		artifact.deleted = Math.abs(Number(m[3]));
	}
	return artifact;
}

export function classifyDiffLine(line: string): DiffLineKind {
	if (line.startsWith('+++') || line.startsWith('---')) return 'meta';
	if (line.startsWith('@@')) return 'hunk';
	if (line.startsWith('+')) return 'add';
	if (line.startsWith('-')) return 'del';
	if (line.startsWith(' ') || line === '') return 'ctx';
	return 'other';
}

export function isEditFileTool(toolName: string | undefined | null): boolean {
	const name = (toolName ?? '').trim().toLowerCase();
	return name === 'edit_file' || name.endsWith('__edit_file') || name.endsWith('/edit_file');
}

/** Whether tool output should render via EditDiffBlock. */
export function shouldRenderEditDiff(
	toolName: string | undefined | null,
	output: string | undefined | null
): boolean {
	if (!output) return false;
	return isEditFileTool(toolName) || looksLikeDiffArtifact(output);
}
