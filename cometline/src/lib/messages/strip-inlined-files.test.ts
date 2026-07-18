import { describe, expect, it } from 'vitest';
import { stripInlinedFileBlocks } from './strip-inlined-files';

describe('stripInlinedFileBlocks', () => {
	it('removes CometMind inlined file blocks', () => {
		const input = 'review @AGENTS.md\n\n[File: AGENTS.md]\n```\n# Title\nbody\n```';
		expect(stripInlinedFileBlocks(input)).toBe('review @AGENTS.md');
	});

	it('removes blocks when the file contains nested code fences', () => {
		const input = [
			'review @AGENTS.md',
			'',
			'[File: AGENTS.md]',
			'```',
			'# AGENTS.md',
			'',
			'```bash',
			'make install',
			'```',
			'',
			'more text',
			'```'
		].join('\n');
		expect(stripInlinedFileBlocks(input)).toBe('review @AGENTS.md');
	});

	it('removes dropped-file blocks with language fences', () => {
		const input = 'check this\n\n[File: example.md]\n````markdown\n# Hello\n````';
		expect(stripInlinedFileBlocks(input)).toBe('check this');
	});

	it('removes missing-file error notes', () => {
		const input =
			'look at @missing.go\n\n<!-- Could not include missing.go: file not found -->';
		expect(stripInlinedFileBlocks(input)).toBe('look at @missing.go');
	});

	it('leaves plain text unchanged', () => {
		expect(stripInlinedFileBlocks('hello @world')).toBe('hello @world');
	});

	it('removes path-only reference stubs', () => {
		const input = [
			'review @main.go and @src/lib/',
			'',
			'[Referenced file: main.go — use read_file (or other tools) if you need contents; do not assume body is attached]',
			'',
			'[Referenced directory: src/lib/ — use list_dir/glob/grep as needed]'
		].join('\n');
		expect(stripInlinedFileBlocks(input)).toBe('review @main.go and @src/lib/');
	});
});
