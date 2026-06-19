import { describe, expect, it } from 'vitest';
import { codemirrorLanguageSupport } from './codemirror-language';

describe('codemirrorLanguageSupport', () => {
	it('returns language extensions for supported languages', () => {
		expect(codemirrorLanguageSupport('typescript')).toHaveLength(1);
		expect(codemirrorLanguageSupport('bash')).toHaveLength(1);
		expect(codemirrorLanguageSupport('svelte')).toHaveLength(1);
	});

	it('falls back to plain text for unknown languages', () => {
		expect(codemirrorLanguageSupport('diff')).toEqual([]);
		expect(codemirrorLanguageSupport(null)).toEqual([]);
	});
});
