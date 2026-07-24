import type { BundledLanguage, SpecialLanguage, ThemedToken } from 'shiki';
import { CODE_THEME, getHighlighter, resolveLanguage } from '$lib/markdown/highlight';
import type { DiffLineKind } from '$lib/tools/diff-artifact';
import type { GitDiffLine } from './git-diff-lines';

export type HighlightedDiffLine = {
	kind: DiffLineKind;
	/** Original unified-diff line text (selection / copy). */
	text: string;
	/** Leading marker for content lines: +, -, space, or empty. */
	prefix: string;
	/** Escaped HTML for the line body (syntax-colored when possible). */
	html: string;
};

function escapeHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;');
}

function tokensToHtml(tokens: ThemedToken[]): string {
	return tokens
		.map((token) => {
			const content = escapeHtml(token.content);
			if (!token.color) return content;
			return `<span style="color:${escapeHtml(token.color)}">${content}</span>`;
		})
		.join('');
}

/** Split a classified unified-diff line into marker + body. */
export function splitDiffContentLine(
	kind: DiffLineKind,
	text: string
): { prefix: string; body: string } {
	if (kind === 'add' || kind === 'del' || kind === 'ctx') {
		if (text.startsWith('+') || text.startsWith('-') || text.startsWith(' ')) {
			return { prefix: text[0] ?? '', body: text.slice(1) };
		}
	}
	return { prefix: '', body: text };
}

/**
 * Highlight unified-diff lines with language grammar for the file.
 *
 * Reconstructs approximate old/new sides so multi-line constructs highlight
 * better than pure per-line tokenization. Falls back to escaped plaintext
 * when the language is unknown or Shiki fails.
 */
export async function highlightGitDiffLines(
	lines: GitDiffLine[],
	language: string | null | undefined
): Promise<HighlightedDiffLine[]> {
	const split = lines.map((line) => {
		const { prefix, body } = splitDiffContentLine(line.kind, line.text);
		return { ...line, prefix, body };
	});

	const oldBodies: string[] = [];
	const newBodies: string[] = [];
	const oldIndex: Array<number | null> = [];
	const newIndex: Array<number | null> = [];

	for (const line of split) {
		if (line.kind === 'del' || line.kind === 'ctx') {
			oldIndex.push(oldBodies.length);
			oldBodies.push(line.body);
		} else {
			oldIndex.push(null);
		}
		if (line.kind === 'add' || line.kind === 'ctx') {
			newIndex.push(newBodies.length);
			newBodies.push(line.body);
		} else {
			newIndex.push(null);
		}
	}

	let oldTokenLines: ThemedToken[][] | null = null;
	let newTokenLines: ThemedToken[][] | null = null;

	try {
		const highlighter = await getHighlighter();
		const resolved = resolveLanguage(highlighter, language ?? undefined) as
			| BundledLanguage
			| SpecialLanguage
			| null;
		if (resolved) {
			if (oldBodies.length > 0) {
				oldTokenLines = highlighter.codeToTokens(oldBodies.join('\n'), {
					lang: resolved,
					theme: CODE_THEME
				}).tokens;
			}
			if (newBodies.length > 0) {
				newTokenLines = highlighter.codeToTokens(newBodies.join('\n'), {
					lang: resolved,
					theme: CODE_THEME
				}).tokens;
			}
		}
	} catch {
		oldTokenLines = null;
		newTokenLines = null;
	}

	return split.map((line, i) => {
		if (line.kind === 'meta' || line.kind === 'hunk' || line.kind === 'other') {
			return {
				kind: line.kind,
				text: line.text,
				prefix: '',
				html: escapeHtml(line.text)
			};
		}

		let tokens: ThemedToken[] | null = null;
		if (line.kind === 'del') {
			const idx = oldIndex[i];
			tokens = idx != null ? (oldTokenLines?.[idx] ?? null) : null;
		} else if (line.kind === 'add' || line.kind === 'ctx') {
			const idx = newIndex[i];
			tokens = idx != null ? (newTokenLines?.[idx] ?? null) : null;
		}

		const html = tokens ? tokensToHtml(tokens) : escapeHtml(line.body);
		return {
			kind: line.kind,
			text: line.text,
			prefix: line.prefix,
			html
		};
	});
}
