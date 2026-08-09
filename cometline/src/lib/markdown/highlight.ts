import { createHighlighterCore, type HighlighterCore } from 'shiki/core';
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript';
import bash from 'shiki/langs/bash.mjs';
import css from 'shiki/langs/css.mjs';
import diff from 'shiki/langs/diff.mjs';
import go from 'shiki/langs/go.mjs';
import html from 'shiki/langs/html.mjs';
import javascript from 'shiki/langs/javascript.mjs';
import json from 'shiki/langs/json.mjs';
import jsx from 'shiki/langs/jsx.mjs';
import markdown from 'shiki/langs/markdown.mjs';
import python from 'shiki/langs/python.mjs';
import rust from 'shiki/langs/rust.mjs';
import shellscript from 'shiki/langs/shellscript.mjs';
import sql from 'shiki/langs/sql.mjs';
import svelte from 'shiki/langs/svelte.mjs';
import tsx from 'shiki/langs/tsx.mjs';
import typescript from 'shiki/langs/typescript.mjs';
import yaml from 'shiki/langs/yaml.mjs';
import githubLight from 'shiki/themes/github-light.mjs';

/**
 * Languages we pre-load for assistant code blocks. Shiki may auto-load extra
 * grammar dependencies (e.g. embedded languages) beyond this list.
 */
export const SUPPORTED_LANGUAGES = [
	'typescript',
	'javascript',
	'tsx',
	'jsx',
	'svelte',
	'css',
	'html',
	'json',
	'markdown',
	'bash',
	'shellscript',
	'go',
	'python',
	'yaml',
	'diff',
	'sql',
	'rust'
] as const;

/** The single light theme used for code blocks; matches the app's light UI. */
export const CODE_THEME = 'github-light';

let highlighterPromise: Promise<HighlighterCore> | null = null;

/**
 * Returns a process-wide singleton Shiki highlighter. Initialization is async
 * (loads TextMate grammars) but happens only once. We use the JavaScript regex
 * engine so no Oniguruma WASM binary is required in the browser/Electron
 * renderer.
 */
export function getHighlighter(): Promise<HighlighterCore> {
	if (!highlighterPromise) {
		highlighterPromise = createHighlighterCore({
			themes: [githubLight],
			langs: [
				typescript,
				javascript,
				tsx,
				jsx,
				svelte,
				css,
				html,
				json,
				markdown,
				bash,
				shellscript,
				go,
				python,
				yaml,
				diff,
				sql,
				rust
			],
			engine: createJavaScriptRegexEngine({ forgiving: true })
		}).catch((err) => {
			// Reset so a later call can retry instead of caching a rejected promise.
			highlighterPromise = null;
			throw err;
		});
	}
	return highlighterPromise;
}

/**
 * Normalizes a fenced-code language hint to a grammar Shiki knows about, or
 * returns null when we have no grammar (caller should fall back to plaintext).
 */
export function resolveLanguage(
	highlighter: HighlighterCore,
	lang: string | undefined
): string | null {
	if (!lang) return null;
	const normalized = lang.trim().toLowerCase();
	if (!normalized) return null;
	const aliases: Record<string, string> = {
		sh: 'bash',
		shell: 'bash',
		zsh: 'bash',
		js: 'javascript',
		ts: 'typescript',
		py: 'python',
		yml: 'yaml',
		md: 'markdown',
		golang: 'go',
		rs: 'rust'
	};
	const resolved = aliases[normalized] ?? normalized;
	const loaded = new Set(highlighter.getLoadedLanguages());
	return loaded.has(resolved) ? resolved : null;
}
