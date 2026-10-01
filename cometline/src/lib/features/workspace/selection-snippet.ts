export type SelectionLineRange = {
	startLine: number;
	endLine: number;
};

export type FileSnippetContext = {
	kind: 'file';
	title: string;
	source: string;
	content: string;
};

const SOURCE_LINE_SELECTOR = '[data-source-start-line][data-source-end-line]';

function boundaryElement(container: Node, offset: number, isStart: boolean): Element | null {
	if (container.nodeType === Node.TEXT_NODE) return container.parentElement;
	if (!(container instanceof Element)) return null;
	const childIndex = isStart ? offset : offset - 1;
	const child = container.childNodes.item(
		Math.max(0, Math.min(childIndex, container.childNodes.length - 1))
	);
	if (!child) return container;
	if (child.nodeType === Node.TEXT_NODE) return child.parentElement;
	return child instanceof Element ? child : container;
}

function sourceRangeForBoundary(
	container: Node,
	offset: number,
	isStart: boolean,
	root: Element
): SelectionLineRange | null {
	let element = boundaryElement(container, offset, isStart);
	if (!element || !root.contains(element)) return null;
	if (!element.matches(SOURCE_LINE_SELECTOR)) {
		element = element.closest(SOURCE_LINE_SELECTOR);
	}
	if (!element || !root.contains(element)) return null;
	const startLine = Number(element.getAttribute('data-source-start-line'));
	const endLine = Number(element.getAttribute('data-source-end-line'));
	if (
		!Number.isInteger(startLine) ||
		!Number.isInteger(endLine) ||
		startLine < 1 ||
		endLine < startLine
	) {
		return null;
	}
	return { startLine, endLine };
}

/** Derive source lines from rendered Markdown annotations without matching text. */
export function sourceLineRangeFromDomRange(
	range: Range,
	root: Element
): SelectionLineRange | null {
	if (range.collapsed) return null;
	const start = sourceRangeForBoundary(range.startContainer, range.startOffset, true, root);
	const end = sourceRangeForBoundary(range.endContainer, range.endOffset, false, root);
	if (!start || !end) return null;
	return {
		startLine: Math.min(start.startLine, end.startLine),
		endLine: Math.max(start.endLine, end.endLine)
	};
}

/** 1-based line numbers covering the character range [start, end) in `source`. */
export function lineRangeFromOffsets(
	source: string,
	start: number,
	end: number
): SelectionLineRange {
	const from = Math.max(0, Math.min(start, end, source.length));
	const to = Math.max(from, Math.min(Math.max(start, end), source.length));
	let line = 1;
	let startLine = 1;
	let endLine = 1;
	for (let i = 0; i < source.length; i++) {
		if (i === from) startLine = line;
		if (i < to) endLine = line;
		if (source[i] === '\n') line += 1;
	}
	if (from === to) {
		endLine = startLine;
	}
	return { startLine, endLine };
}

function normalizeForMatch(text: string): string {
	return text.replace(/\s+/g, ' ').trim();
}

/**
 * Locate selected preview text inside the original source. Prefers an exact
 * match; falls back to whitespace-normalized match. Returns null when ambiguous
 * or missing.
 */
export function findSelectionInSource(
	source: string,
	selectedText: string
): { start: number; end: number } | null {
	const selected = selectedText.trim();
	if (!selected || !source) return null;

	const exactIndex = source.indexOf(selected);
	if (exactIndex >= 0) {
		const second = source.indexOf(selected, exactIndex + 1);
		if (second >= 0) return null;
		return { start: exactIndex, end: exactIndex + selected.length };
	}

	const needle = normalizeForMatch(selected);
	if (!needle) return null;

	const sourceNorm = normalizeForMatch(source);
	const normIndex = sourceNorm.indexOf(needle);
	if (normIndex < 0) return null;
	if (sourceNorm.indexOf(needle, normIndex + 1) >= 0) return null;

	// Map normalized offset back approximately by walking source whitespace.
	let si = 0;
	let ni = 0;
	let start = -1;
	while (si < source.length && ni <= normIndex + needle.length) {
		const ch = source[si];
		if (/\s/.test(ch)) {
			if (ni < sourceNorm.length && sourceNorm[ni] === ' ') {
				if (ni === normIndex) start = si;
				ni += 1;
			}
			while (si + 1 < source.length && /\s/.test(source[si + 1])) si += 1;
			si += 1;
			continue;
		}
		if (ni === normIndex) start = si;
		ni += 1;
		si += 1;
		if (ni === normIndex + needle.length) {
			return start >= 0 ? { start, end: si } : null;
		}
	}
	return null;
}

export function fileSnippetSource(filePath: string, range: SelectionLineRange | null): string {
	const base = filePath.startsWith('@runtime/wiki/')
		? filePath
		: filePath.startsWith('workspace-file:')
			? filePath
			: `workspace-file:${filePath}`;
	if (!range) return base;
	return `${base}#L${range.startLine}-L${range.endLine}`;
}

export function fileSnippetTitle(filePath: string, range: SelectionLineRange | null): string {
	const name = filePath.split(/[/\\]/).filter(Boolean).pop() || filePath;
	if (!range) return name;
	if (range.startLine === range.endLine) return `${name}:${range.startLine}`;
	return `${name}:${range.startLine}-${range.endLine}`;
}

function snippetBaseSource(filePath: string): string {
	if (filePath.startsWith('@runtime/wiki/') || filePath.startsWith('workspace-file:')) {
		return filePath;
	}
	return `workspace-file:${filePath}`;
}

export function buildFileSnippetContext(opts: {
	filePath: string;
	selectedText: string;
	sourceText?: string;
	lineRange?: SelectionLineRange | null;
}): FileSnippetContext | null {
	const content = opts.selectedText.trim();
	if (!content) return null;

	let range = opts.lineRange ?? null;
	if (!range && opts.sourceText) {
		const match = findSelectionInSource(opts.sourceText, content);
		if (match) range = lineRangeFromOffsets(opts.sourceText, match.start, match.end);
	}

	const base = snippetBaseSource(opts.filePath);
	const labelPath = base.replace(/^workspace-file:/, '');

	return {
		kind: 'file',
		title: fileSnippetTitle(labelPath, range),
		source: fileSnippetSource(base, range),
		content: content.slice(0, 50000)
	};
}
