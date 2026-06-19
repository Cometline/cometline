import { describe, expect, it } from 'vitest';
import {
	formatDroppedFiles,
	isSupportedTextFile,
	languageForFilename,
	readDroppedTextFiles,
	type DroppedFileLike
} from './dropped-files';

function file(name: string, text: string, opts: { type?: string; size?: number } = {}): DroppedFileLike {
	return {
		name,
		type: opts.type ?? 'text/plain',
		size: opts.size ?? text.length,
		text: () => Promise.resolve(text)
	};
}

describe('dropped-files', () => {
	it('detects supported text files by MIME type or extension', () => {
		expect(isSupportedTextFile(file('notes.unknown', 'hello', { type: 'text/plain' }))).toBe(true);
		expect(isSupportedTextFile(file('main.go', 'package main', { type: '' }))).toBe(true);
		expect(isSupportedTextFile(file('photo.png', '', { type: 'image/png' }))).toBe(false);
	});

	it('maps filenames to fence languages', () => {
		expect(languageForFilename('component.svelte')).toBe('svelte');
		expect(languageForFilename('script.ts')).toBe('typescript');
		expect(languageForFilename('README.md')).toBe('markdown');
	});

	it('reads accepted files and rejects unsupported or oversized files', async () => {
		const result = await readDroppedTextFiles(
			[
				file('a.txt', 'hello'),
				file('image.png', '', { type: 'image/png' }),
				file('large.md', 'x', { size: 10 })
			],
			{ maxBytes: 5 }
		);

		expect(result.accepted).toEqual([{ name: 'a.txt', language: 'text', text: 'hello' }]);
		expect(result.rejected).toEqual([
			{ name: 'image.png', reason: 'Unsupported file type.' },
			{ name: 'large.md', reason: 'File is larger than 1 KB.' }
		]);
	});

	it('formats dropped files with a safe markdown fence', () => {
		const formatted = formatDroppedFiles([
			{ name: 'example.md', language: 'markdown', text: 'before\n```\nafter\n' }
		]);

		expect(formatted).toBe('[File: example.md]\n````markdown\nbefore\n```\nafter\n````');
	});
});
