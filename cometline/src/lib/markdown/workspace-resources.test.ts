// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import {
	dirnamePosix,
	hydrateWorkspaceMarkdownImages,
	joinPosixSafe,
	resolveWorkspaceMarkdownPath,
	rewriteLocalResourcesInHtml
} from './workspace-resources';

describe('dirnamePosix / joinPosixSafe', () => {
	it('dirname for root and nested files', () => {
		expect(dirnamePosix('README.md')).toBe('');
		expect(dirnamePosix('docs/guide.md')).toBe('docs');
		expect(dirnamePosix('a/b/c.md')).toBe('a/b');
	});

	it('joins and rejects escape above root', () => {
		expect(joinPosixSafe('docs', '../static/x.png')).toBe('static/x.png');
		expect(joinPosixSafe('', './static/preview.png')).toBe('static/preview.png');
		expect(joinPosixSafe('', '../secret.png')).toBeNull();
		expect(joinPosixSafe('docs', '../../x.png')).toBeNull();
	});
});

describe('resolveWorkspaceMarkdownPath', () => {
	it('resolves relative paths against the markdown file directory', () => {
		expect(resolveWorkspaceMarkdownPath('./static/preview.png', 'README.md')).toBe(
			'static/preview.png'
		);
		expect(resolveWorkspaceMarkdownPath('../assets/a.png', 'docs/guide.md')).toBe(
			'assets/a.png'
		);
		expect(resolveWorkspaceMarkdownPath('images/x.png', 'docs/guide.md')).toBe(
			'docs/images/x.png'
		);
	});

	it('treats leading-slash paths as workspace-root relative', () => {
		expect(resolveWorkspaceMarkdownPath('/static/minako.png', 'docs/a.md')).toBe(
			'static/minako.png'
		);
	});

	it('ignores remote schemes and pure anchors', () => {
		expect(resolveWorkspaceMarkdownPath('https://example.com/a.png', 'README.md')).toBeNull();
		expect(resolveWorkspaceMarkdownPath('mailto:a@b.com', 'README.md')).toBeNull();
		expect(resolveWorkspaceMarkdownPath('#section', 'README.md')).toBeNull();
	});
});

describe('rewriteLocalResourcesInHtml', () => {
	it('rewrites local images and links', () => {
		const html = rewriteLocalResourcesInHtml(
			'<p><img src="./static/preview.png" alt="Minako"><a href="./cometline/docs/ollama-local.md">docs</a></p>',
			'README.md',
			'workspace'
		);
		expect(html).toContain('data-workspace-src="static/preview.png"');
		expect(html).toContain('data-file-path="cometline/docs/ollama-local.md"');
		expect(html).toContain('class="md-workspace-link"');
		expect(html).not.toContain('href="./cometline');
		expect(html).not.toContain('src="./static');
	});

	it('prefixes wiki open paths', () => {
		const html = rewriteLocalResourcesInHtml(
			'<a href="./notes.md">n</a>',
			'index.md',
			'wiki'
		);
		expect(html).toContain('data-file-path="@runtime/wiki/notes.md"');
	});

	it('leaves remote urls alone', () => {
		const html = rewriteLocalResourcesInHtml(
			'<a href="https://example.com">ex</a><img src="https://cdn.example/a.png">',
			'README.md',
			'workspace'
		);
		expect(html).toContain('href="https://example.com"');
		expect(html).toContain('src="https://cdn.example/a.png"');
		expect(html).not.toContain('data-workspace-src');
		expect(html).not.toContain('data-file-path');
	});
});

describe('hydrateWorkspaceMarkdownImages', () => {
	it('replaces data-workspace-src with data urls', async () => {
		const readFile = vi.fn(async () => ({
			kind: 'image' as const,
			data_url: 'data:image/png;base64,abc'
		}));
		const html = await hydrateWorkspaceMarkdownImages(
			'<img alt="Minako" data-workspace-src="static/preview.png" src="">',
			readFile
		);
		expect(readFile).toHaveBeenCalledWith('static/preview.png');
		expect(html).toContain('src="data:image/png;base64,abc"');
		expect(html).not.toContain('data-workspace-src');
	});

	it('dedupes fetches for the same path', async () => {
		const readFile = vi.fn(async () => ({
			kind: 'image' as const,
			data_url: 'data:image/png;base64,abc'
		}));
		await hydrateWorkspaceMarkdownImages(
			'<img data-workspace-src="a.png"><img data-workspace-src="a.png">',
			readFile
		);
		expect(readFile).toHaveBeenCalledTimes(1);
	});
});
