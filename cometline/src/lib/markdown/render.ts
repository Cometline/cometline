import {
	Marked,
	type RendererObject,
	type Token,
	type Tokens,
	type TokensList,
	type TokenizerAndRendererExtension
} from 'marked';
import remend from 'remend';
import DOMPurify from 'dompurify';
import katex from 'katex';
import { getHighlighter, resolveLanguage, CODE_THEME } from './highlight';
import {
	buildBrokenWikilinkChip,
	buildEmbedChip,
	buildFileEmbedChip,
	buildSkillEmbedChip,
	findNextUserTextToken
} from './embed';
import { stripInlinedFileBlocks } from '$lib/messages/strip-inlined-files';
import { toWikiUiPath } from '$lib/wiki/paths';
import { parseWikilinkInner, resolveWikilink } from '$lib/wiki/wikilinks';
import {
	hydrateWorkspaceMarkdownImages,
	rewriteLocalResourcesInHtml,
	type WorkspaceMarkdownResources
} from './workspace-resources';

export type { WorkspaceMarkdownResources };

/** Wiki file list used while the current `renderMarkdown` call is in flight. */
let activeWikiFiles: readonly string[] = [];
let katexCssPromise: Promise<void> | null = null;

function ensureKatexCss(): Promise<void> {
	if (typeof document === 'undefined') return Promise.resolve();
	if (!katexCssPromise) {
		katexCssPromise = import('katex/dist/katex.min.css')
			.then(() => undefined)
			.catch((error) => {
				katexCssPromise = null;
				console.error('KaTeX CSS failed to load', error);
			});
	}
	return katexCssPromise;
}

/** Escapes text for safe inclusion in HTML. */
function escapeHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

/** Wrap fenced `<pre>` with a copy button (styled/handled in AssistantMarkdown). */
function wrapCodeBlock(preHtml: string, attributes = ''): string {
	return `<div class="md-code-block"${attributes}><button type="button" class="md-code-copy" data-code-copy aria-label="Copy code"></button>${preHtml}</div>`;
}

/**
 * Highlights a fenced code block to HTML. Uses the shared Shiki highlighter when
 * a grammar is available, otherwise falls back to an escaped plaintext block.
 * The Shiki output is a `<pre class="shiki">...<code>...</code></pre>` string
 * with inline token colors, which the sanitizer is configured to allow.
 */
async function highlightCodeBlock(text: string, lang: string | undefined): Promise<string> {
	try {
		const highlighter = await getHighlighter();
		const resolved = resolveLanguage(highlighter, lang);
		if (resolved) {
			return wrapCodeBlock(
				highlighter.codeToHtml(text, { lang: resolved, theme: CODE_THEME })
			);
		}
	} catch {
		// Fall through to the plaintext fallback below.
	}
	const langClass = lang ? ` class="language-${escapeHtml(lang)}"` : '';
	return wrapCodeBlock(
		`<pre class="shiki shiki-plain"><code${langClass}>${escapeHtml(text)}</code></pre>`
	);
}

/** Renders a LaTeX string to KaTeX HTML, falling back to escaped source on error. */
function renderMath(tex: string, displayMode: boolean): string {
	try {
		return katex.renderToString(tex, {
			displayMode,
			throwOnError: false,
			output: 'html'
		});
	} catch {
		const wrapper = displayMode ? 'div' : 'span';
		return `<${wrapper} class="math-error">${escapeHtml(displayMode ? `$$${tex}$$` : `$${tex}$`)}</${wrapper}>`;
	}
}

/** Block math: `$$ ... $$`. Must be checked before inline math. */
const blockMathExtension: TokenizerAndRendererExtension = {
	name: 'blockMath',
	level: 'block',
	start(src: string) {
		return src.indexOf('$$');
	},
	tokenizer(src: string) {
		const match = /^\$\$([\s\S]+?)\$\$/.exec(src);
		if (!match) return undefined;
		return {
			type: 'blockMath',
			raw: match[0],
			text: match[1].trim()
		};
	},
	renderer(token) {
		return renderMath(token.text, true);
	}
};

/** Inline math: `$ ... $`. Avoids matching currency by requiring non-space edges. */
const inlineMathExtension: TokenizerAndRendererExtension = {
	name: 'inlineMath',
	level: 'inline',
	start(src: string) {
		const index = src.indexOf('$');
		return index < 0 ? undefined : index;
	},
	tokenizer(src: string) {
		// Single $...$ with no surrounding whitespace inside the delimiters, and
		// not a $$ block. Disallow newlines so prose dollar signs stay literal.
		const match = /^\$(?!\$)((?:[^$\n]|\\\$)+?)\$/.exec(src);
		if (!match) return undefined;
		const inner = match[1];
		if (/^\s|\s$/.test(inner)) return undefined;
		return {
			type: 'inlineMath',
			raw: match[0],
			text: inner.trim()
		};
	},
	renderer(token) {
		return renderMath(token.text, false);
	}
};

/**
 * Trailing punctuation that should not be swallowed into an autolinked URL
 * (e.g. the period in "see https://grok.com." or a closing paren in prose).
 */
const URL_TRAILING_PUNCTUATION = /[.,;:!?)\]}'"]+$/;

/**
 * Inline extension: turns a bare http(s) URL into an embed chip. marked's inline
 * tokenizer only feeds plain text runs to extensions, so URLs already inside a
 * markdown link `[text](url)` or inline code never reach here — they stay as
 * normal links/text. This intentionally runs as a custom extension (which marked
 * checks before its built-in GFM autolink) so a bare URL becomes a chip, not a
 * plain `<a>`.
 */
const urlEmbedExtension: TokenizerAndRendererExtension = {
	name: 'urlEmbed',
	level: 'inline',
	start(src: string) {
		const index = src.search(/https?:\/\//);
		return index < 0 ? undefined : index;
	},
	tokenizer(src: string) {
		const match = /^https?:\/\/[^\s<]+/.exec(src);
		if (!match) return undefined;
		let url = match[0];
		// Don't eat trailing sentence punctuation; leave it as following text.
		const trailing = URL_TRAILING_PUNCTUATION.exec(url);
		if (trailing) {
			url = url.slice(0, url.length - trailing[0].length);
		}
		if (!url) return undefined;
		return {
			type: 'urlEmbed',
			raw: url,
			text: url
		};
	},
	renderer(token) {
		return buildEmbedChip(token.text);
	}
};

/**
 * Inline extension: turns `@runtime/wiki/...` into a clickable file chip in
 * assistant markdown. Runs before GFM autolink so wiki paths stay file previews.
 */
const wikiFileEmbedExtension: TokenizerAndRendererExtension = {
	name: 'wikiFileEmbed',
	level: 'inline',
	start(src: string) {
		const index = src.indexOf('@runtime/wiki/');
		return index < 0 ? undefined : index;
	},
	tokenizer(src: string) {
		const match = /^@runtime\/wiki\/[A-Za-z0-9_][A-Za-z0-9_./-]*/.exec(src);
		if (!match) return undefined;
		return {
			type: 'wikiFileEmbed',
			raw: match[0],
			text: match[0]
		};
	},
	renderer(token) {
		return buildFileEmbedChip(token.text);
	}
};

/**
 * Inline extension: Obsidian-style `[[Page]]` / `[[Page|alias]]` → file chips
 * when a wiki file index is supplied to `renderMarkdown`.
 */
const wikilinkEmbedExtension: TokenizerAndRendererExtension = {
	name: 'wikilinkEmbed',
	level: 'inline',
	start(src: string) {
		const index = src.indexOf('[[');
		return index < 0 ? undefined : index;
	},
	tokenizer(src: string) {
		const match = /^\[\[([^\]]+)\]\]/.exec(src);
		if (!match) return undefined;
		const parsed = parseWikilinkInner(match[1] ?? '');
		if (!parsed) return undefined;
		return {
			type: 'wikilinkEmbed',
			raw: match[0],
			target: parsed.target,
			alias: parsed.alias ?? ''
		};
	},
	renderer(token) {
		const target = String((token as { target?: string }).target ?? '');
		const alias = String((token as { alias?: string }).alias ?? '').trim();
		const label = alias || target;
		const resolved = resolveWikilink(target, activeWikiFiles);
		if (!resolved) return buildBrokenWikilinkChip(label);
		return buildFileEmbedChip(toWikiUiPath(resolved), label);
	}
};

/** Per-render cache of pre-highlighted code HTML, keyed by code token text. */
type CodeHtmlCache = Map<string, string>;

type SourceLineRange = {
	startLine: number;
	endLine: number;
};

const tokenSourceLines = new WeakMap<object, SourceLineRange>();

function newlineCount(value: string): number {
	return value.split('\n').length - 1;
}

function sourceRange(startLine: number, raw: string): SourceLineRange {
	const trailingNewline = raw.endsWith('\n') ? 1 : 0;
	return {
		startLine,
		endLine: Math.max(startLine, startLine + newlineCount(raw) - trailingNewline)
	};
}

function annotateTokenSequence(tokens: Token[], raw: string, startLine: number): void {
	let cursor = 0;
	for (const token of tokens) {
		const tokenRaw = token.raw ?? '';
		let offset = raw.indexOf(tokenRaw, cursor);
		if (offset < 0) offset = raw.indexOf(tokenRaw);
		if (offset < 0) offset = cursor;
		const tokenStartLine = startLine + newlineCount(raw.slice(0, offset));
		tokenSourceLines.set(token, sourceRange(tokenStartLine, tokenRaw));

		if ('items' in token && Array.isArray(token.items)) {
			annotateTokenSequence(token.items, tokenRaw, tokenStartLine);
		} else if ('header' in token && Array.isArray(token.header) && 'rows' in token) {
			annotateTableTokens(token as Tokens.Table, tokenStartLine);
		} else if ('tokens' in token && Array.isArray(token.tokens)) {
			annotateTokenSequence(token.tokens, tokenRaw, tokenStartLine);
		}
		cursor = offset + tokenRaw.length;
	}
}

function annotateTableRow(cells: Tokens.TableCell[], raw: string, startLine: number): void {
	let cursor = 0;
	for (const cell of cells) {
		const firstRaw = cell.tokens[0]?.raw ?? cell.text;
		let offset = raw.indexOf(firstRaw, cursor);
		if (offset < 0) offset = raw.indexOf(firstRaw);
		if (offset < 0) offset = cursor;
		tokenSourceLines.set(cell, sourceRange(startLine, raw));
		annotateTokenSequence(cell.tokens, raw.slice(offset), startLine);
		cursor = offset + firstRaw.length;
	}
}

function annotateTableTokens(table: Tokens.Table, startLine: number): void {
	const lines = table.raw.split('\n');
	annotateTableRow(table.header, lines[0] ?? '', startLine);
	for (let index = 0; index < table.rows.length; index += 1) {
		annotateTableRow(table.rows[index], lines[index + 2] ?? '', startLine + index + 2);
	}
}

function annotateSourceLines(tokens: Token[] | TokensList): Token[] | TokensList {
	annotateTokenSequence(tokens, tokens.map((token) => token.raw).join(''), 1);
	return tokens;
}

function sourceLineAttributes(token: object): string {
	const range = tokenSourceLines.get(token);
	if (!range) return '';
	return ` data-source-start-line="${range.startLine}" data-source-end-line="${range.endLine}"`;
}

function stripSourceLineAttributes(html: string): string {
	return html.replace(
		/\sdata-source-(?:start|end)-line(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+))?/gi,
		''
	);
}

function renderAnnotatedText(
	this: { parser: { parseInline(tokens: Token[]): string } },
	token: Tokens.Text | Tokens.Escape
): string {
	if ('tokens' in token && token.tokens) return this.parser.parseInline(token.tokens);
	const range = tokenSourceLines.get(token);
	if (!range || token.raw !== token.text || !token.raw.includes('\n')) {
		const text = 'escaped' in token && token.escaped ? token.text : escapeHtml(token.text);
		return `<span${sourceLineAttributes(token)}>${text}</span>`;
	}
	return token.text
		.split('\n')
		.map((line, index) => {
			const lineNumber = range.startLine + index;
			return `<span data-source-start-line="${lineNumber}" data-source-end-line="${lineNumber}">${escapeHtml(line)}</span>`;
		})
		.join('\n');
}

const CODE_CACHE_MAX = 128;
let activeRenderCodeKeys: Set<string> | null = null;

function pruneCodeCache() {
	if (!activeRenderCodeKeys) return;
	for (const key of codeCache.keys()) {
		if (!activeRenderCodeKeys.has(key)) codeCache.delete(key);
	}
	while (codeCache.size > CODE_CACHE_MAX) {
		const first = codeCache.keys().next().value;
		if (first === undefined) break;
		codeCache.delete(first);
	}
}

/**
 * Builds a Marked instance with GFM, raw HTML escaped, and Shiki code blocks.
 *
 * Marked renderers must be synchronous, so highlighting happens in an async
 * `walkTokens` pass that populates `codeCache`; the synchronous `code` renderer
 * then reads the pre-rendered HTML out of that cache.
 */
function createMarkedInstance(codeCache: CodeHtmlCache, annotateSource = false): Marked {
	const marked = new Marked({
		async: true,
		gfm: true,
		breaks: false
	});
	const renderer: RendererObject = {
		code(token: Tokens.Code) {
			const key = `${token.lang ?? ''}\u0000${token.text}`;
			const highlighted = codeCache.get(key);
			if (highlighted) {
				return annotateSource
					? highlighted.replace(
							'<div class="md-code-block"',
							`<div class="md-code-block"${sourceLineAttributes(token)}`
						)
					: highlighted;
			}
			const preHtml = `<pre class="shiki shiki-plain"><code>${escapeHtml(token.text)}</code></pre>`;
			return wrapCodeBlock(preHtml, annotateSource ? sourceLineAttributes(token) : '');
		}
	};
	if (annotateSource) {
		Object.assign(renderer, {
			heading(
				this: { parser: { parseInline(tokens: Token[]): string } },
				token: Tokens.Heading
			) {
				return `<h${token.depth}${sourceLineAttributes(token)}>${this.parser.parseInline(token.tokens)}</h${token.depth}>\n`;
			},
			paragraph(
				this: { parser: { parseInline(tokens: Token[]): string } },
				token: Tokens.Paragraph
			) {
				return `<p${sourceLineAttributes(token)}>${this.parser.parseInline(token.tokens)}</p>\n`;
			},
			strong(
				this: { parser: { parseInline(tokens: Token[]): string } },
				token: Tokens.Strong
			) {
				return `<strong${sourceLineAttributes(token)}>${this.parser.parseInline(token.tokens)}</strong>`;
			},
			em(this: { parser: { parseInline(tokens: Token[]): string } }, token: Tokens.Em) {
				return `<em${sourceLineAttributes(token)}>${this.parser.parseInline(token.tokens)}</em>`;
			},
			link(this: { parser: { parseInline(tokens: Token[]): string } }, token: Tokens.Link) {
				const title = token.title ? ` title="${escapeHtml(token.title)}"` : '';
				return `<a href="${escapeHtml(token.href)}"${title}${sourceLineAttributes(token)}>${this.parser.parseInline(token.tokens)}</a>`;
			},
			codespan(token: Tokens.Codespan) {
				return `<code${sourceLineAttributes(token)}>${escapeHtml(token.text)}</code>`;
			},
			html(token: Tokens.HTML | Tokens.Tag) {
				return stripSourceLineAttributes(token.text);
			},
			text: renderAnnotatedText
		} satisfies RendererObject);
	}

	marked.use({
		extensions: [
			blockMathExtension,
			inlineMathExtension,
			wikiFileEmbedExtension,
			wikilinkEmbedExtension,
			urlEmbedExtension
		],
		async walkTokens(token) {
			if (token.type !== 'code') return;
			const code = token as Tokens.Code;
			const key = `${code.lang ?? ''}\u0000${code.text}`;
			activeRenderCodeKeys?.add(key);
			if (!codeCache.has(key)) {
				codeCache.set(key, await highlightCodeBlock(code.text, code.lang));
			}
		},
		// Raw/inline HTML is passed through to DOMPurify's safe-tag allowlist.
		renderer
	});
	if (annotateSource) {
		marked.use({ hooks: { processAllTokens: annotateSourceLines } });
	}

	return marked;
}

const codeCache: CodeHtmlCache = new Map();
const markedInstance = createMarkedInstance(codeCache);
const sourceAnnotatedMarkedInstance = createMarkedInstance(codeCache, true);

/** Safe URL schemes allowed on links in rendered markdown. */
const SAFE_LINK_SCHEMES = /^(https?:|mailto:)/i;

let domPurifyConfigured = false;

/**
 * Configures DOMPurify once: force external links to open via the app's
 * external-link handler and drop unsafe URL schemes. Runs only in the browser.
 */
function ensureDomPurifyHooks(): void {
	if (domPurifyConfigured) return;
	if (typeof window === 'undefined') return;
	domPurifyConfigured = true;

	DOMPurify.addHook('afterSanitizeAttributes', (node) => {
		if (node.nodeName === 'A' && node instanceof HTMLElement) {
			const href = node.getAttribute('href');
			if (href && !SAFE_LINK_SCHEMES.test(href)) {
				node.removeAttribute('href');
			} else if (href) {
				node.setAttribute('target', '_blank');
				node.setAttribute('rel', 'noopener noreferrer');
				node.setAttribute('data-external-link', href);
			}
		}
	});
}

/** DOMPurify allowlist tuned for markdown output plus Shiki's styled spans. */
const SANITIZE_CONFIG = {
	ALLOWED_TAGS: [
		'a',
		'b',
		'blockquote',
		'br',
		'button',
		'code',
		'del',
		'div',
		'em',
		'h1',
		'h2',
		'h3',
		'h4',
		'h5',
		'h6',
		'hr',
		'i',
		'img',
		'input',
		'kbd',
		'li',
		'mark',
		'ol',
		'p',
		'pre',
		'span',
		'strong',
		'sup',
		'sub',
		'table',
		'tbody',
		'td',
		'th',
		'thead',
		'tr',
		'u',
		'ul',
		'#text'
	],
	ALLOWED_ATTR: [
		'href',
		'title',
		'src',
		'alt',
		'class',
		'style',
		'target',
		'rel',
		'data-external-link',
		'data-embed-url',
		'data-file-path',
		'data-workspace-src',
		'data-skill-name',
		'data-code-copy',
		'role',
		'tabindex',
		'width',
		'height',
		'loading',
		'type',
		'checked',
		'disabled',
		'aria-hidden',
		'aria-label'
	],
	FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form'],
	ALLOW_DATA_ATTR: false
};

const SOURCE_SANITIZE_CONFIG = {
	...SANITIZE_CONFIG,
	ALLOWED_ATTR: [
		...SANITIZE_CONFIG.ALLOWED_ATTR,
		'data-source-start-line',
		'data-source-end-line'
	]
};

/**
 * Renders streaming markdown to sanitized HTML.
 *
 * Pipeline: remend (heal incomplete inline markdown) → marked (GFM parse with a
 * Shiki code renderer, raw HTML escaped) → DOMPurify (strict allowlist). The
 * returned HTML is safe to inject via Svelte `{@html}`.
 */
export type RenderMarkdownOptions = {
	/** Wiki-root-relative `.md` paths used to resolve `[[wikilinks]]`. */
	wikiFiles?: readonly string[];
	/**
	 * When previewing a workspace/wiki markdown file, resolve relative image and
	 * link paths against that file and hydrate images via the content API.
	 */
	workspaceResources?: WorkspaceMarkdownResources;
	/** Add source line metadata for rendered file-preview selections. */
	annotateSourceLines?: boolean;
};

export async function renderMarkdown(
	source: string,
	options?: RenderMarkdownOptions
): Promise<string> {
	if (!source) return '';
	ensureDomPurifyHooks();
	activeRenderCodeKeys = new Set();
	activeWikiFiles = options?.wikiFiles ?? [];
	try {
		const healed = remend(source);
		const parser = options?.annotateSourceLines
			? sourceAnnotatedMarkedInstance
			: markedInstance;
		let rawHtml = await parser.parse(healed);
		if (rawHtml.includes('class="katex')) await ensureKatexCss();
		const resources = options?.workspaceResources;
		if (resources) {
			rawHtml = rewriteLocalResourcesInHtml(rawHtml, resources.filePath, resources.kind);
		}
		pruneCodeCache();
		const sanitizeConfig = options?.annotateSourceLines
			? SOURCE_SANITIZE_CONFIG
			: SANITIZE_CONFIG;
		let sanitized = DOMPurify.sanitize(rawHtml, sanitizeConfig);
		if (resources) {
			sanitized = await hydrateWorkspaceMarkdownImages(sanitized, resources.readFile);
		}
		return sanitized;
	} finally {
		activeRenderCodeKeys = null;
		activeWikiFiles = [];
	}
}

/**
 * Renders a user message as literal text (NOT markdown), turning bare http(s)
 * URLs, @file mentions, and /skill commands into embed chips. Everything else
 * is HTML-escaped so the user's text is shown verbatim.
 */
export function renderUserText(source: string): string {
	if (!source) return '';
	ensureDomPurifyHooks();

	const displayText = stripInlinedFileBlocks(source);
	let out = '';
	let index = 0;
	while (index < displayText.length) {
		const token = findNextUserTextToken(displayText, index);
		if (!token || token.index > index) {
			const end = token?.index ?? displayText.length;
			out += escapeHtml(displayText.slice(index, end));
			index = end;
			if (!token) break;
			continue;
		}

		if (token.type === 'url') {
			out += buildEmbedChip(token.url);
			if (token.urlSuffix) out += escapeHtml(token.urlSuffix);
		} else if (token.type === 'file') {
			out += buildFileEmbedChip(token.path);
		} else {
			if (token.leading) out += escapeHtml(token.leading);
			out += buildSkillEmbedChip(token.name);
		}
		index = token.index + token.length;
	}

	return DOMPurify.sanitize(out, SANITIZE_CONFIG);
}
