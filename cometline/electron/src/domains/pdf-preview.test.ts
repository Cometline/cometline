import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
	PDF_PREVIEW_MAX_BYTES,
	PDF_PREVIEW_SCHEME,
	createPdfPreviewRegistry,
	registerPdfPreviewProtocol
} from './pdf-preview.js';

const temporaryDirectories: string[] = [];

afterEach(() => {
	for (const directory of temporaryDirectories.splice(0))
		fs.rmSync(directory, { recursive: true, force: true });
});

function fixture() {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-pdf-preview-'));
	const workspace = path.join(root, 'workspace');
	const wiki = path.join(root, 'wiki');
	fs.mkdirSync(workspace);
	fs.mkdirSync(wiki);
	fs.writeFileSync(path.join(workspace, 'report.pdf'), '%PDF-1.7');
	fs.writeFileSync(path.join(workspace, 'notes.txt'), 'nope');
	fs.writeFileSync(path.join(wiki, 'paper.pdf'), '%PDF-1.7');
	temporaryDirectories.push(root);
	return { root, workspace, wiki };
}

describe('PDF preview registry', () => {
	it('creates exact-file workspace and wiki grants without exposing local paths', async () => {
		const { workspace, wiki } = fixture();
		const registry = createPdfPreviewRegistry({
			fs,
			path,
			wikiRoot: wiki,
			randomUUID: () => 'token'
		});

		const workspaceResult = await registry.create({
			scope: 'workspace',
			workspacePath: workspace,
			relativePath: 'report.pdf'
		});
		expect(workspaceResult).toEqual({
			ok: true,
			token: 'token',
			url: `${PDF_PREVIEW_SCHEME}://pdf/token/report.pdf`
		});
		expect(JSON.stringify(workspaceResult)).not.toContain(workspace);

		const wikiResult = await registry.create({ scope: 'wiki', relativePath: 'paper.pdf' });
		expect(wikiResult.ok).toBe(true);
	});

	it('rejects non-PDF files, traversal, symlink escapes, and oversized PDFs', async () => {
		const { root, workspace, wiki } = fixture();
		const outside = path.join(root, 'outside.pdf');
		fs.writeFileSync(outside, '%PDF');
		fs.symlinkSync(outside, path.join(workspace, 'link.pdf'));
		fs.writeFileSync(path.join(workspace, 'large.pdf'), '');
		fs.truncateSync(path.join(workspace, 'large.pdf'), PDF_PREVIEW_MAX_BYTES + 1);
		const registry = createPdfPreviewRegistry({ fs, path, wikiRoot: wiki });

		await expect(
			registry.create({
				scope: 'workspace',
				workspacePath: 'relative/workspace',
				relativePath: 'report.pdf'
			})
		).resolves.toEqual({ ok: false, error: 'Invalid preview root' });
		await expect(
			registry.create({
				scope: 'workspace',
				workspacePath: workspace,
				relativePath: 'notes.txt'
			})
		).resolves.toEqual({ ok: false, error: 'Only PDF files can use PDF preview' });
		await expect(
			registry.create({
				scope: 'workspace',
				workspacePath: workspace,
				relativePath: '../outside.pdf'
			})
		).resolves.toEqual({ ok: false, error: 'Path escapes workspace' });
		await expect(
			registry.create({
				scope: 'workspace',
				workspacePath: workspace,
				relativePath: 'link.pdf'
			})
		).resolves.toEqual({ ok: false, error: 'Path escapes workspace' });
		await expect(
			registry.create({
				scope: 'workspace',
				workspacePath: workspace,
				relativePath: 'large.pdf'
			})
		).resolves.toEqual({ ok: false, error: 'PDF exceeds 50 MB preview limit' });
	});

	it('expires and revokes grants', async () => {
		const { workspace, wiki } = fixture();
		let now = 100;
		const registry = createPdfPreviewRegistry({
			fs,
			path,
			wikiRoot: wiki,
			now: () => now,
			randomUUID: () => 'token'
		});
		await registry.create({
			scope: 'workspace',
			workspacePath: workspace,
			relativePath: 'report.pdf'
		});
		expect(registry.resolve('token')?.absolutePath).toBe(
			fs.realpathSync(path.join(workspace, 'report.pdf'))
		);
		registry.revoke('token');
		expect(registry.resolve('token')).toBeNull();
		await registry.create({
			scope: 'workspace',
			workspacePath: workspace,
			relativePath: 'report.pdf'
		});
		now += 31 * 60 * 1000;
		expect(registry.resolve('token')).toBeNull();
	});
});

describe('PDF preview protocol', () => {
	it('serves a granted file and rejects altered paths and methods', async () => {
		const { workspace, wiki } = fixture();
		const registry = createPdfPreviewRegistry({
			fs,
			path,
			wikiRoot: wiki,
			randomUUID: () => 'token'
		});
		const result = await registry.create({
			scope: 'workspace',
			workspacePath: workspace,
			relativePath: 'report.pdf'
		});
		if (!result.ok) throw new Error(result.error);

		let handler: ((request: Request) => Response | Promise<Response>) | undefined;
		const fetch = vi.fn(
			async () => new Response('%PDF', { headers: { 'content-type': 'application/pdf' } })
		);
		registerPdfPreviewProtocol({
			registry,
			fs,
			path,
			net: { fetch },
			protocol: {
				handle: (_scheme, next) => {
					handler = next as typeof handler;
				}
			}
		});

		const response = await handler!(new Request(result.url));
		expect(response.status).toBe(200);
		expect(response.headers.get('content-type')).toBe('application/pdf');
		expect(fetch).toHaveBeenCalledWith(
			pathToFileURL(fs.realpathSync(path.join(workspace, 'report.pdf'))).toString(),
			expect.any(Object)
		);
		expect(
			(await handler!(new Request(`${PDF_PREVIEW_SCHEME}://pdf/token/other.pdf`))).status
		).toBe(404);
		expect((await handler!(new Request(result.url, { method: 'POST' }))).status).toBe(405);
	});

	it('returns 404 when a granted file is removed before serving', async () => {
		const { workspace, wiki } = fixture();
		const registry = createPdfPreviewRegistry({
			fs,
			path,
			wikiRoot: wiki,
			randomUUID: () => 'token'
		});
		const result = await registry.create({
			scope: 'workspace',
			workspacePath: workspace,
			relativePath: 'report.pdf'
		});
		if (!result.ok) throw new Error(result.error);
		fs.rmSync(path.join(workspace, 'report.pdf'));

		let handler: ((request: Request) => Response | Promise<Response>) | undefined;
		registerPdfPreviewProtocol({
			registry,
			fs,
			path,
			net: { fetch: vi.fn() },
			protocol: {
				handle: (_scheme, next) => {
					handler = next as typeof handler;
				}
			}
		});

		const response = await handler!(new Request(result.url));
		expect(response.status).toBe(404);
		expect(await response.text()).toBe('PDF preview file unavailable');
	});
});
