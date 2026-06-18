// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { renderMarkdown, renderUserText } from './render';

describe('renderMarkdown', () => {
	it('returns empty string for empty input', async () => {
		expect(await renderMarkdown('')).toBe('');
	});

	it('renders a paragraph', async () => {
		const html = await renderMarkdown('Hello world');
		expect(html).toContain('<p>Hello world</p>');
	});

	it('renders bold and italic', async () => {
		const html = await renderMarkdown('This is **bold** and *italic*.');
		expect(html).toContain('<strong>bold</strong>');
		expect(html).toContain('<em>italic</em>');
	});

	it('renders unordered lists', async () => {
		const html = await renderMarkdown('- one\n- two');
		expect(html).toContain('<ul>');
		expect(html).toContain('<li>one</li>');
		expect(html).toContain('<li>two</li>');
	});

	it('renders GFM tables', async () => {
		const html = await renderMarkdown('| a | b |\n| - | - |\n| 1 | 2 |');
		expect(html).toContain('<table>');
		expect(html).toContain('<th>a</th>');
		expect(html).toContain('<td>1</td>');
	});

	it('renders fenced code blocks as pre/code', async () => {
		const html = await renderMarkdown('```ts\nconst a: number = 1\n```');
		expect(html).toContain('<pre');
		expect(html).toContain('<code');
		// Shiki tokenizes into colored spans for a known language.
		expect(html).toContain('<span');
	});

	it('falls back to escaped plaintext for unknown languages', async () => {
		const html = await renderMarkdown('```unknownlang\n<b>x</b>\n```');
		expect(html).toContain('<pre');
		expect(html).toContain('&lt;b&gt;x&lt;/b&gt;');
	});

	it('heals incomplete inline markdown (streaming)', async () => {
		const html = await renderMarkdown('partial **bold');
		// remend closes the dangling ** so it renders as bold rather than literal.
		expect(html).toContain('<strong>bold</strong>');
	});

	it('strips raw HTML script injection', async () => {
		const html = await renderMarkdown('hello <script>alert(1)</script> world');
		expect(html).not.toContain('<script>');
		expect(html.toLowerCase()).not.toContain('alert(1)</script>');
	});

	it('strips event-handler attributes from allowed raw HTML', async () => {
		const html = await renderMarkdown('<img src="x" onerror="alert(1)">');
		// <img> is allowed, but DOMPurify removes the onerror handler.
		expect(html).not.toContain('onerror');
	});

	it('allows safe inline HTML tags', async () => {
		const html = await renderMarkdown(
			'press <kbd>Ctrl</kbd> and <u>underline</u> <mark>hi</mark>'
		);
		expect(html).toContain('<kbd>Ctrl</kbd>');
		expect(html).toContain('<u>underline</u>');
		expect(html).toContain('<mark>hi</mark>');
	});

	it('drops disallowed raw HTML tags but keeps text content', async () => {
		const html = await renderMarkdown('<marquee>scroll</marquee>');
		expect(html).not.toContain('<marquee');
		expect(html).toContain('scroll');
	});

	it('renders inline math with KaTeX', async () => {
		const html = await renderMarkdown('Einstein: $E = mc^2$ done');
		expect(html).toContain('class="katex"');
		expect(html).not.toContain('$E = mc^2$');
	});

	it('renders block math with KaTeX', async () => {
		const html = await renderMarkdown('$$\\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}$$');
		expect(html).toContain('katex');
		expect(html).toContain('katex-display');
	});

	it('does not treat bare dollar amounts as math', async () => {
		const html = await renderMarkdown('It costs $5 and $10 total.');
		expect(html).not.toContain('class="katex"');
		expect(html).toContain('$5 and $10');
	});

	it('adds external-link routing attributes to safe links', async () => {
		const html = await renderMarkdown('[site](https://example.com)');
		expect(html).toContain('data-external-link="https://example.com"');
		expect(html).toContain('target="_blank"');
		expect(html).toContain('rel="noopener noreferrer"');
	});

	it('drops javascript: link schemes', async () => {
		const html = await renderMarkdown('[x](javascript:alert(1))');
		expect(html).not.toContain('javascript:');
	});

	it('renders inline code without a language', async () => {
		const html = await renderMarkdown('use `npm install` here');
		expect(html).toContain('<code>npm install</code>');
	});

	it('renders a bare URL as an embed chip', async () => {
		const html = await renderMarkdown('see https://grok.com for more');
		expect(html).toContain('class="link-embed"');
		expect(html).toContain('data-embed-url="https://grok.com"');
		expect(html).toContain('grok.com');
	});

	it('does not chip a URL inside a markdown link', async () => {
		const html = await renderMarkdown('[Grok](https://grok.com)');
		expect(html).not.toContain('link-embed');
		expect(html).toContain('>Grok</a>');
	});

	it('does not chip a URL inside inline code', async () => {
		const html = await renderMarkdown('run `curl https://grok.com`');
		expect(html).not.toContain('link-embed');
		expect(html).toContain('<code>');
	});

	it('keeps trailing sentence punctuation outside the chip', async () => {
		const html = await renderMarkdown('visit https://grok.com.');
		expect(html).toContain('data-embed-url="https://grok.com"');
		// The trailing period is not part of the embedded URL.
		expect(html).not.toContain('grok.com.ico'.replace('grok.com', 'grok.com.'));
	});

	it('keeps embed-chip attributes through sanitization', async () => {
		const html = await renderMarkdown('https://grok.com');
		expect(html).toContain('data-embed-url=');
		expect(html).toContain('loading="lazy"');
		expect(html).toContain('width="14"');
	});
});

describe('renderUserText', () => {
	it('returns empty string for empty input', () => {
		expect(renderUserText('')).toBe('');
	});

	it('keeps plain text literal (no markdown formatting)', () => {
		const html = renderUserText('plain *text* and #notheading');
		expect(html).not.toContain('<em>');
		expect(html).not.toContain('<h1>');
		expect(html).toContain('*text*');
		expect(html).toContain('#notheading');
	});

	it('turns a bare URL into an embed chip', () => {
		const html = renderUserText('check https://grok.com please');
		expect(html).toContain('class="link-embed"');
		expect(html).toContain('data-embed-url="https://grok.com"');
		expect(html).toContain('check ');
		expect(html).toContain(' please');
	});

	it('escapes HTML in user text', () => {
		const html = renderUserText('<script>alert(1)</script>');
		expect(html).not.toContain('<script>');
		expect(html).toContain('&lt;script&gt;');
	});

	it('preserves the trailing period after a URL', () => {
		const html = renderUserText('go to https://grok.com.');
		expect(html).toContain('data-embed-url="https://grok.com"');
		expect(html).toMatch(/<\/a>\.?/);
	});

	it('turns @file mentions into embed chips', () => {
		const html = renderUserText('review @src/lib/foo.ts please');
		expect(html).toContain('class="file-embed"');
		expect(html).toContain('data-file-path="src/lib/foo.ts"');
		expect(html).toContain('review ');
		expect(html).toContain(' please');
	});

	it('turns slash skills into embed chips', () => {
		const html = renderUserText('run /model gpt-4');
		expect(html).toContain('class="skill-embed"');
		expect(html).toContain('/model');
	});

	it('hides persisted CometMind file inlines but keeps @ mentions', () => {
		const html = renderUserText(
			[
				'review @AGENTS.md',
				'',
				'[File: AGENTS.md]',
				'```',
				'# AGENTS.md',
				'',
				'```bash',
				'make install',
				'```',
				'secret',
				'```'
			].join('\n')
		);
		expect(html).toContain('class="file-embed"');
		expect(html).toContain('AGENTS.md');
		expect(html).not.toContain('secret');
		expect(html).not.toContain('[File:');
	});
});
