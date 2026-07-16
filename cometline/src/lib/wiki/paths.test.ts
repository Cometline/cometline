import { describe, expect, it } from 'vitest';
import {
	isWikiReadOnlyPath,
	isWikiUiPath,
	toWikiRelative,
	toWikiUiPath,
	WIKI_RUNTIME_PREFIX
} from './paths';

describe('wiki paths', () => {
	it('detects wiki UI paths', () => {
		expect(isWikiUiPath('@runtime/wiki/index.md')).toBe(true);
		expect(isWikiUiPath('README.md')).toBe(false);
	});

	it('converts between UI and relative paths', () => {
		expect(toWikiRelative('@runtime/wiki/entities/foo.md')).toBe('entities/foo.md');
		expect(toWikiUiPath('index.md')).toBe(`${WIKI_RUNTIME_PREFIX}index.md`);
	});

	it('marks raw and schema paths read-only', () => {
		expect(isWikiReadOnlyPath('@runtime/wiki/raw/note.md')).toBe(true);
		expect(isWikiReadOnlyPath('@runtime/wiki/WIKI.md')).toBe(true);
		expect(isWikiReadOnlyPath('@runtime/wiki/entities/foo.md')).toBe(false);
	});
});
