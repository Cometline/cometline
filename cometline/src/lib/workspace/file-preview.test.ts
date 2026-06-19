import { describe, expect, it } from 'vitest';
import { extensionFromPath, isImagePath, isMarkdownPath, languageFromPath } from './file-preview';

describe('file-preview helpers', () => {
	it('detects language from extension', () => {
		expect(languageFromPath('src/lib/foo.ts')).toBe('typescript');
		expect(languageFromPath('README.md')).toBe('markdown');
	});

	it('detects markdown and image paths', () => {
		expect(isMarkdownPath('docs/guide.md')).toBe(true);
		expect(isImagePath('static/logo.png')).toBe(true);
		expect(isImagePath('src/main.go')).toBe(false);
	});

	it('extracts extension from nested paths', () => {
		expect(extensionFromPath('src/components/App.svelte')).toBe('.svelte');
	});
});
