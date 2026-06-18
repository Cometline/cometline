import type { Extension } from '@codemirror/state';
import { StreamLanguage } from '@codemirror/language';
import { css } from '@codemirror/lang-css';
import { go } from '@codemirror/lang-go';
import { html } from '@codemirror/lang-html';
import { javascript } from '@codemirror/lang-javascript';
import { json } from '@codemirror/lang-json';
import { markdown } from '@codemirror/lang-markdown';
import { python } from '@codemirror/lang-python';
import { rust } from '@codemirror/lang-rust';
import { sql } from '@codemirror/lang-sql';
import { yaml } from '@codemirror/lang-yaml';
import { svelte } from '@replit/codemirror-lang-svelte';
import { shell } from '@codemirror/legacy-modes/mode/shell';

export function codemirrorLanguageSupport(language: string | null): Extension[] {
	switch (language) {
		case 'typescript':
			return [javascript({ typescript: true })];
		case 'tsx':
			return [javascript({ typescript: true, jsx: true })];
		case 'javascript':
			return [javascript()];
		case 'jsx':
			return [javascript({ jsx: true })];
		case 'svelte':
			return [svelte()];
		case 'css':
			return [css()];
		case 'html':
			return [html()];
		case 'json':
			return [json()];
		case 'markdown':
			return [markdown()];
		case 'yaml':
		case 'toml':
			return [yaml()];
		case 'go':
			return [go()];
		case 'python':
			return [python()];
		case 'rust':
			return [rust()];
		case 'sql':
			return [sql()];
		case 'bash':
			return [StreamLanguage.define(shell)];
		default:
			return [];
	}
}
