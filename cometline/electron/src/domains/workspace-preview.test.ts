import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import { readWorkspaceFileForPreview } from './workspace-preview.js';

const temporaryDirectories: string[] = [];

afterEach(() => {
	for (const directory of temporaryDirectories.splice(0)) {
		fs.rmSync(directory, { force: true, recursive: true });
	}
});

function createWorkspace() {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-workspace-preview-'));
	const workspace = path.join(root, 'workspace');
	fs.mkdirSync(workspace);
	temporaryDirectories.push(root);
	return { root, workspace };
}

function preview(workspacePath: string, relativePath: string) {
	return readWorkspaceFileForPreview({ fs, path }, workspacePath, relativePath);
}

describe('readWorkspaceFileForPreview', () => {
	it('rejects paths that escape the workspace', async () => {
		const { root, workspace } = createWorkspace();
		fs.writeFileSync(path.join(root, 'private.txt'), 'private');

		await expect(preview(workspace, '../private.txt')).resolves.toEqual({
			ok: false,
			error: 'Path escapes workspace'
		});
	});

	it('rejects binary and oversized text files', async () => {
		const { workspace } = createWorkspace();
		fs.writeFileSync(path.join(workspace, 'binary.dat'), Buffer.from([0x66, 0x00, 0x6f]));
		fs.writeFileSync(path.join(workspace, 'large.txt'), 'x'.repeat(256 * 1024 + 1));

		await expect(preview(workspace, 'binary.dat')).resolves.toEqual({
			ok: false,
			error: 'Binary file cannot be previewed'
		});
		await expect(preview(workspace, 'large.txt')).resolves.toEqual({
			ok: false,
			error: 'File exceeds 256 KB preview limit'
		});
	});

	it('returns text content and bounded image data URLs', async () => {
		const { workspace } = createWorkspace();
		fs.writeFileSync(path.join(workspace, 'notes.md'), '# Notes\n');
		fs.writeFileSync(path.join(workspace, 'pixel.png'), Buffer.from([0x89, 0x50, 0x4e, 0x47]));

		await expect(preview(workspace, 'notes.md')).resolves.toEqual({
			ok: true,
			kind: 'text',
			content: '# Notes\n',
			extension: '.md'
		});
		await expect(preview(workspace, 'pixel.png')).resolves.toEqual({
			ok: true,
			kind: 'image',
			mimeType: 'image/png',
			dataUrl: 'data:image/png;base64,iVBORw=='
		});
	});
});
