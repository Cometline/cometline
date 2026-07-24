import { describe, expect, it } from 'vitest';
import { materialIconNameForPath } from './material-file-icon';

describe('materialIconNameForPath', () => {
	it('maps common extensions', () => {
		expect(materialIconNameForPath('src/app.ts')).toBe('typescript');
		expect(materialIconNameForPath('main.go')).toBe('go');
		expect(materialIconNameForPath('App.svelte')).toBe('svelte');
		expect(materialIconNameForPath('notes.md')).toBe('markdown');
		expect(materialIconNameForPath('data.json')).toBe('json');
	});

	it('prefers exact file-name matches', () => {
		expect(materialIconNameForPath('package.json')).toBe('nodejs');
		expect(materialIconNameForPath('README.md')).toBe('readme');
	});

	it('matches compound extensions like test.ts', () => {
		expect(materialIconNameForPath('foo.test.ts')).toBe('test-ts');
	});

	it('falls back to the generic file icon', () => {
		expect(materialIconNameForPath('weird.notarealext123')).toBe('file');
	});
});
