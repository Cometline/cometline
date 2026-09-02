import { describe, expect, it } from 'vitest';
import {
	extensionFromPath,
	isImagePath,
	isMarkdownPath,
	isPdfPath,
	languageFromPath,
	shouldSkipTextPreviewReload
} from './file-preview';

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

	it('detects PDF paths case-insensitively', () => {
		expect(isPdfPath('docs/report.pdf')).toBe(true);
		expect(isPdfPath('docs/REPORT.PDF')).toBe(true);
		expect(isPdfPath('docs/report.pdf.txt')).toBe(false);
		expect(isPdfPath('report')).toBe(false);
	});

	it('extracts extension from nested paths', () => {
		expect(extensionFromPath('src/components/App.svelte')).toBe('.svelte');
	});

	it('skips a keep-view reload when on-disk text matches the open buffer', () => {
		expect(shouldSkipTextPreviewReload(true, 'text', 'same', 'same')).toBe(true);
		expect(shouldSkipTextPreviewReload(true, 'text', 'old', 'new')).toBe(false);
		expect(shouldSkipTextPreviewReload(false, 'text', 'same', 'same')).toBe(false);
		expect(shouldSkipTextPreviewReload(true, 'image', 'same', 'same')).toBe(false);
	});
});
