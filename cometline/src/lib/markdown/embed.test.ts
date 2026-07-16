import { describe, expect, it } from 'vitest';
import {
	domainFromUrl,
	faviconUrl,
	isHttpUrl,
	buildEmbedChip,
	buildFileEmbedChip,
	buildSkillEmbedChip,
	canonicalFileMentionPath,
	extractUrls,
	findNextUserTextToken,
	fileLabelFromPath,
	fileMentionText,
	linkifyRuntimeWikiMentions
} from './embed';

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

describe('extractUrls', () => {
	it('returns an empty array for empty input', () => {
		expect(extractUrls('')).toEqual([]);
	});

	it('extracts a single URL', () => {
		expect(extractUrls('check https://grok.com here')).toEqual(['https://grok.com']);
	});

	it('extracts multiple URLs in order', () => {
		expect(extractUrls('https://a.com then https://b.com')).toEqual([
			'https://a.com',
			'https://b.com'
		]);
	});

	it('dedups repeated URLs', () => {
		expect(extractUrls('https://a.com and again https://a.com')).toEqual(['https://a.com']);
	});

	it('trims trailing sentence punctuation', () => {
		expect(extractUrls('see https://grok.com.')).toEqual(['https://grok.com']);
	});

	it('ignores non-http text', () => {
		expect(extractUrls('just plain words, no links')).toEqual([]);
	});
});

describe('fileLabelFromPath', () => {
	it('returns the basename', () => {
		expect(fileLabelFromPath('src/lib/foo.ts')).toBe('foo.ts');
	});
});

describe('fileMentionText', () => {
	it('prefixes workspace-relative paths with @', () => {
		expect(fileMentionText('README.md')).toBe('@README.md');
	});

	it('does not double-prefix @runtime wiki paths', () => {
		expect(fileMentionText('@runtime/wiki/index.md')).toBe('@runtime/wiki/index.md');
	});
});

describe('canonicalFileMentionPath', () => {
	it('keeps workspace paths without a leading @', () => {
		expect(canonicalFileMentionPath('@src/lib/foo.ts')).toBe('src/lib/foo.ts');
	});

	it('preserves @runtime/wiki paths for preview routing', () => {
		expect(canonicalFileMentionPath('@runtime/wiki/index.md')).toBe('@runtime/wiki/index.md');
		expect(canonicalFileMentionPath('runtime/wiki/index.md')).toBe('@runtime/wiki/index.md');
	});
});

describe('buildFileEmbedChip', () => {
	it('renders a clickable file chip with data-file-path', () => {
		const html = buildFileEmbedChip('src/lib/foo.ts');
		expect(html).toContain('class="file-embed"');
		expect(html).toContain('data-file-path="src/lib/foo.ts"');
		expect(html).toContain('@foo.ts');
	});

	it('keeps wiki paths clickable with the runtime prefix', () => {
		const html = buildFileEmbedChip('@runtime/wiki/index.md');
		expect(html).toContain('data-file-path="@runtime/wiki/index.md"');
		expect(html).toContain('@runtime/wiki/index.md');
	});
});

describe('buildSkillEmbedChip', () => {
	it('renders a skill chip label', () => {
		const html = buildSkillEmbedChip('create-skill');
		expect(html).toContain('class="skill-embed"');
		expect(html).toContain('/create-skill');
	});
});

describe('findNextUserTextToken', () => {
	it('prefers the earliest token', () => {
		const token = findNextUserTextToken('see @src/a.ts and https://a.com', 0);
		expect(token?.type).toBe('file');
	});

	it('does not treat email addresses as file mentions', () => {
		const token = findNextUserTextToken('email user@domain.com', 0);
		expect(token).toBeNull();
	});

	it('canonicalizes @runtime/wiki mentions for preview routing', () => {
		const token = findNextUserTextToken('see @runtime/wiki/index.md', 0);
		expect(token?.type).toBe('file');
		if (token?.type === 'file') {
			expect(token.path).toBe('@runtime/wiki/index.md');
		}
	});
});

describe('linkifyRuntimeWikiMentions', () => {
	it('wraps wiki paths in file embed chips', () => {
		const html = linkifyRuntimeWikiMentions('Updated @runtime/wiki/index.md for you.');
		expect(html).toContain('class="file-embed"');
		expect(html).toContain('data-file-path="@runtime/wiki/index.md"');
	});
});
