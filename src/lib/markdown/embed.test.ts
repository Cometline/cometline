import { describe, expect, it } from 'vitest';
import { domainFromUrl, faviconUrl, isHttpUrl, buildEmbedChip } from './embed';

describe('domainFromUrl', () => {
	it('returns the hostname', () => {
		expect(domainFromUrl('https://grok.com')).toBe('grok.com');
	});

	it('strips a leading www.', () => {
		expect(domainFromUrl('https://www.example.com/path?q=1')).toBe('example.com');
	});

	it('keeps subdomains other than www', () => {
		expect(domainFromUrl('https://docs.example.com')).toBe('docs.example.com');
	});

	it('returns the input when not a valid URL', () => {
		expect(domainFromUrl('not a url')).toBe('not a url');
	});
});

describe('isHttpUrl', () => {
	it('accepts http and https', () => {
		expect(isHttpUrl('http://a.com')).toBe(true);
		expect(isHttpUrl('https://a.com')).toBe(true);
	});

	it('rejects other schemes', () => {
		expect(isHttpUrl('javascript:alert(1)')).toBe(false);
		expect(isHttpUrl('mailto:a@b.com')).toBe(false);
		expect(isHttpUrl('ftp://a.com')).toBe(false);
	});
});

describe('faviconUrl', () => {
	it('builds a DuckDuckGo favicon URL for the domain', () => {
		expect(faviconUrl('https://www.grok.com/x')).toBe(
			'https://icons.duckduckgo.com/ip3/grok.com.ico'
		);
	});
});

describe('buildEmbedChip', () => {
	it('renders an anchor with favicon and domain label', () => {
		const html = buildEmbedChip('https://grok.com');
		expect(html).toContain('class="link-embed"');
		expect(html).toContain('href="https://grok.com"');
		expect(html).toContain('data-embed-url="https://grok.com"');
		expect(html).toContain('icons.duckduckgo.com/ip3/grok.com.ico');
		expect(html).toContain('>grok.com</span>');
	});

	it('uses a custom label when provided', () => {
		const html = buildEmbedChip('https://grok.com', 'Grok');
		expect(html).toContain('>Grok</span>');
	});

	it('omits href for non-http URLs but still escapes', () => {
		const html = buildEmbedChip('javascript:alert(1)');
		expect(html).not.toContain('href=');
	});

	it('escapes HTML in the URL', () => {
		const html = buildEmbedChip('https://a.com/"><img>');
		expect(html).not.toContain('"><img>');
		expect(html).toContain('&quot;&gt;&lt;img&gt;');
	});
});
