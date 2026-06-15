import { Marked, type Tokens, type TokenizerAndRendererExtension } from 'marked';
import remend from 'remend';
import DOMPurify from 'dompurify';
import katex from 'katex';
import { getHighlighter, resolveLanguage, CODE_THEME } from './highlight';
import { buildEmbedChip } from './embed';

/** Escapes text for safe inclusion in HTML. */
function escapeHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
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
			return highlighter.codeToHtml(text, { lang: resolved, theme: CODE_THEME });
		}
	} catch {
		// Fall through to the plaintext fallback below.
	}
	const langClass = lang ? ` class="language-${escapeHtml(lang)}"` : '';
	return `<pre class="shiki shiki-plain"><code${langClass}>${escapeHtml(text)}</code></pre>`;
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

/** Per-render cache of pre-highlighted code HTML, keyed by code token text. */
type CodeHtmlCache = Map<string, string>;

/**
 * Builds a Marked instance with GFM, raw HTML escaped, and Shiki code blocks.
 *
 * Marked renderers must be synchronous, so highlighting happens in an async
 * `walkTokens` pass that populates `codeCache`; the synchronous `code` renderer
 * then reads the pre-rendered HTML out of that cache.
 */
function createMarkedInstance(codeCache: CodeHtmlCache): Marked {
	const marked = new Marked({
		async: true,
		gfm: true,
		breaks: false
	});

	marked.use({
		extensions: [blockMathExtension, inlineMathExtension, urlEmbedExtension],
		async walkTokens(token) {
			if (token.type !== 'code') return;
			const code = token as Tokens.Code;
			const key = `${code.lang ?? ''}\u0000${code.text}`;
			if (!codeCache.has(key)) {
				codeCache.set(key, await highlightCodeBlock(code.text, code.lang));
			}
		},
		renderer: {
			code({ text, lang }: Tokens.Code) {
				const key = `${lang ?? ''}\u0000${text}`;
				return (
					codeCache.get(key) ??
					`<pre class="shiki shiki-plain"><code>${escapeHtml(text)}</code></pre>`
				);
			}
			// Raw/inline HTML is intentionally NOT escaped here: it is passed through
			// to DOMPurify, which strips everything except the safe-tag allowlist in
			// SANITIZE_CONFIG. This lets benign tags like <u>/<kbd> render while
			// still blocking <script>, event handlers, and unsafe URLs.
		}
	});

	return marked;
}

const codeCache: CodeHtmlCache = new Map();
const markedInstance = createMarkedInstance(codeCache);

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
		'width',
		'height',
		'loading',
		'type',
		'checked',
		'disabled',
		'aria-hidden'
	],
	FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form'],
	ALLOW_DATA_ATTR: false
};

/**
 * Renders streaming markdown to sanitized HTML.
 *
 * Pipeline: remend (heal incomplete inline markdown) → marked (GFM parse with a
 * Shiki code renderer, raw HTML escaped) → DOMPurify (strict allowlist). The
 * returned HTML is safe to inject via Svelte `{@html}`.
 */
export async function renderMarkdown(source: string): Promise<string> {
	if (!source) return '';
	ensureDomPurifyHooks();
	// Keep the content-keyed code cache bounded; each render only needs the code
	// blocks present in the current source.
	codeCache.clear();
	const healed = remend(source);
	const rawHtml = await markedInstance.parse(healed);
	return DOMPurify.sanitize(rawHtml, SANITIZE_CONFIG);
}

/** Global URL matcher used to linkify plain user text. */
const BARE_URL_GLOBAL = /https?:\/\/[^\s<]+/g;

/**
 * Renders a user message as literal text (NOT markdown), turning bare http(s)
 * URLs into embed chips. Everything except URLs is HTML-escaped so the user's
 * text is shown verbatim — no headings, bold, or other markdown formatting is
 * applied. Newlines are preserved by the caller via `white-space: pre-wrap`.
 */
export function renderUserText(source: string): string {
	if (!source) return '';
	ensureDomPurifyHooks();

	let out = '';
	let lastIndex = 0;
	for (const match of source.matchAll(BARE_URL_GLOBAL)) {
		const start = match.index ?? 0;
		out += escapeHtml(source.slice(lastIndex, start));
		let url = match[0];
		const trailing = URL_TRAILING_PUNCTUATION.exec(url);
		const suffix = trailing ? url.slice(url.length - trailing[0].length) : '';
		if (suffix) url = url.slice(0, url.length - suffix.length);
		out += buildEmbedChip(url);
		if (suffix) out += escapeHtml(suffix);
		lastIndex = start + match[0].length;
	}
	out += escapeHtml(source.slice(lastIndex));

	return DOMPurify.sanitize(out, SANITIZE_CONFIG);
}
