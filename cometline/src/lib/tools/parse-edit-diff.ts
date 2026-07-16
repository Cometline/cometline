export type DiffLineKind = 'meta' | 'hunk' | 'add' | 'del' | 'ctx' | 'other';

export type ParsedDiffLine = {
	kind: DiffLineKind;
	text: string;
};

export type ParsedEditDiff = {
	summary: string;
	lines: ParsedDiffLine[];
};

const BEGIN = '*** Begin Diff';
const END = '*** End Diff';

/** Extract summary + unified-diff lines from edit_file tool output. */
export function parseEditDiff(output: string): ParsedEditDiff | null {
	const text = output ?? '';
	const begin = text.indexOf(BEGIN);
	const end = text.indexOf(END);
	if (begin < 0 || end < 0 || end <= begin) return null;

	const summary = text.slice(0, begin).trim();
	const body = text.slice(begin + BEGIN.length, end).replace(/^\n/, '').replace(/\n$/, '');
	if (!body.trim()) return null;

	const lines: ParsedDiffLine[] = body.split('\n').map((line) => ({
		kind: classifyDiffLine(line),
		text: line
	}));
	return { summary, lines };
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
	return toolName === 'edit_file';
}
