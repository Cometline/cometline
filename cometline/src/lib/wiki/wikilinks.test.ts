import { describe, expect, it } from 'vitest';
import { parseWikilinkInner, resolveWikilink, wikiStemFromPath } from './wikilinks';

describe('wikiStemFromPath', () => {
	it('strips directories and .md / .html', () => {
		expect(wikiStemFromPath('entities/Foo Bar.md')).toBe('Foo Bar');
		expect(wikiStemFromPath('raw/2026-07-16-agentic-reasoning-for-llms.html')).toBe(
			'2026-07-16-agentic-reasoning-for-llms'
		);
	});
});

describe('parseWikilinkInner', () => {
	it('parses plain targets and aliases', () => {
		expect(parseWikilinkInner('Foo')).toEqual({ target: 'Foo' });
		expect(parseWikilinkInner('Foo|label')).toEqual({ target: 'Foo', alias: 'label' });
		expect(parseWikilinkInner('Foo#heading|label')).toEqual({ target: 'Foo', alias: 'label' });
		expect(parseWikilinkInner('Foo#heading')).toEqual({ target: 'Foo' });
	});

	it('rejects empty targets', () => {
		expect(parseWikilinkInner('')).toBeNull();
		expect(parseWikilinkInner('|alias')).toBeNull();
	});
});

describe('resolveWikilink', () => {
	const files = ['index.md', 'entities/runtime-mounts.md', 'concepts/Runtime.md', 'syntheses/overview.md'];

	it('resolves exact relative paths', () => {
		expect(resolveWikilink('entities/runtime-mounts', files)).toBe('entities/runtime-mounts.md');
	});

	it('resolves unique basename stems case-insensitively', () => {
		expect(resolveWikilink('Runtime-Mounts', files)).toBe('entities/runtime-mounts.md');
		expect(resolveWikilink('overview', files)).toBe('syntheses/overview.md');
	});

	it('prefers entities over other folders when stems collide', () => {
		const collision = ['entities/dup.md', 'concepts/dup.md'];
		expect(resolveWikilink('dup', collision)).toBe('entities/dup.md');
	});

	it('returns null when unresolved', () => {
		expect(resolveWikilink('missing', files)).toBeNull();
	});

	it('resolves raw html ingest sources linked with .html suffix', () => {
		const withHtml = [...files, 'raw/2026-07-16-agentic-reasoning-for-llms.html'];
		expect(resolveWikilink('2026-07-16-agentic-reasoning-for-llms.html', withHtml)).toBe(
			'raw/2026-07-16-agentic-reasoning-for-llms.html'
		);
		expect(resolveWikilink('2026-07-16-agentic-reasoning-for-llms', withHtml)).toBe(
			'raw/2026-07-16-agentic-reasoning-for-llms.html'
		);
	});
});
