import type { Net, Protocol } from 'electron';
import crypto from 'node:crypto';
import type fs from 'node:fs';
import type path from 'node:path';
import { pathToFileURL } from 'node:url';

export const PDF_PREVIEW_SCHEME = 'cometline-preview';
export const PDF_PREVIEW_HOST = 'pdf';
export const PDF_PREVIEW_MAX_BYTES = 50 * 1024 * 1024;
const PDF_PREVIEW_TOKEN_TTL_MS = 30 * 60 * 1000;
const PDF_PREVIEW_MAX_TOKENS = 100;

type FileSystem = Pick<typeof fs, 'promises'>;
type PathService = Pick<typeof path, 'extname' | 'isAbsolute' | 'resolve' | 'sep'>;

type PreviewEntry = {
	absolutePath: string;
	root: string;
	publicName: string;
	expiresAt: number;
};

export type PdfPreviewRequest =
	| { scope: 'workspace'; workspacePath: string; relativePath: string }
	| { scope: 'wiki'; relativePath: string };

export type PdfPreviewResult =
	| { ok: true; token: string; url: string }
	| { ok: false; error: string };

export interface PdfPreviewDependencies {
	fs: FileSystem;
	path: PathService;
	wikiRoot: string;
	now?: () => number;
	randomUUID?: () => string;
}

function withinRoot(root: string, target: string, pathService: PathService) {
	return target === root || target.startsWith(`${root}${pathService.sep}`);
}

/** Owns short-lived, exact-file grants used by the PDF protocol. */
export function createPdfPreviewRegistry(dependencies: PdfPreviewDependencies) {
	const entries = new Map<string, PreviewEntry>();
	const now = dependencies.now ?? Date.now;
	const randomUUID = dependencies.randomUUID ?? crypto.randomUUID;

	function prune() {
		const timestamp = now();
		for (const [token, entry] of entries) {
			if (entry.expiresAt <= timestamp) entries.delete(token);
		}
		while (entries.size >= PDF_PREVIEW_MAX_TOKENS) {
			const oldest = entries.keys().next().value;
			if (!oldest) break;
			entries.delete(oldest);
		}
	}

	async function create(request: unknown): Promise<PdfPreviewResult> {
		if (!request || typeof request !== 'object')
			return { ok: false, error: 'Invalid PDF preview request' };
		const input = request as Partial<PdfPreviewRequest> & {
			relativePath?: unknown;
			workspacePath?: unknown;
		};
		const relativePath = String(input.relativePath || '').replace(/^[/\\]+/, '');
		if (!relativePath || !['workspace', 'wiki'].includes(String(input.scope))) {
			return { ok: false, error: 'Invalid PDF preview request' };
		}
		if (dependencies.path.extname(relativePath).toLowerCase() !== '.pdf') {
			return { ok: false, error: 'Only PDF files can use PDF preview' };
		}

		const rootInput =
			input.scope === 'wiki' ? dependencies.wikiRoot : String(input.workspacePath || '');
		if (!rootInput || !dependencies.path.isAbsolute(rootInput)) {
			return { ok: false, error: 'Invalid preview root' };
		}
		const resolvedRoot = dependencies.path.resolve(rootInput);
		if (resolvedRoot === dependencies.path.sep) {
			return { ok: false, error: 'Invalid preview root' };
		}

		try {
			const root = await dependencies.fs.promises.realpath(resolvedRoot);
			const candidate = dependencies.path.resolve(root, relativePath);
			if (!withinRoot(root, candidate, dependencies.path)) {
				return { ok: false, error: `Path escapes ${input.scope}` };
			}
			const absolutePath = await dependencies.fs.promises.realpath(candidate);
			if (!withinRoot(root, absolutePath, dependencies.path)) {
				return { ok: false, error: `Path escapes ${input.scope}` };
			}
			const stat = await dependencies.fs.promises.stat(absolutePath);
			if (!stat.isFile()) return { ok: false, error: 'PDF file not found' };
			if (stat.size > PDF_PREVIEW_MAX_BYTES) {
				return { ok: false, error: 'PDF exceeds 50 MB preview limit' };
			}

			prune();
			const token = randomUUID();
			const publicName = absolutePath.split(dependencies.path.sep).pop() || 'document.pdf';
			entries.set(token, {
				absolutePath,
				root,
				publicName,
				expiresAt: now() + PDF_PREVIEW_TOKEN_TTL_MS
			});
			const fileName = encodeURIComponent(publicName);
			return {
				ok: true,
				token,
				url: `${PDF_PREVIEW_SCHEME}://${PDF_PREVIEW_HOST}/${token}/${fileName}`
			};
		} catch {
			return { ok: false, error: 'PDF file not found' };
		}
	}

	function resolve(token: string): PreviewEntry | null {
		const entry = entries.get(token);
		if (!entry) return null;
		if (entry.expiresAt <= now()) {
			entries.delete(token);
			return null;
		}
		return entry;
	}

	function revoke(token: unknown) {
		entries.delete(String(token || ''));
	}

	return { create, resolve, revoke, clear: () => entries.clear() };
}

export type PdfPreviewRegistry = ReturnType<typeof createPdfPreviewRegistry>;

/** Serves only files granted by the registry, without exposing their real paths. */
export function registerPdfPreviewProtocol(dependencies: {
	protocol: Pick<Protocol, 'handle'>;
	net: Pick<Net, 'fetch'>;
	fs: FileSystem;
	path: PathService;
	registry: PdfPreviewRegistry;
}) {
	dependencies.protocol.handle(PDF_PREVIEW_SCHEME, async (request) => {
		if (request.method !== 'GET' && request.method !== 'HEAD') {
			return new Response('Method not allowed', { status: 405 });
		}
		const url = new URL(request.url);
		if (url.host !== PDF_PREVIEW_HOST) return new Response('Not found', { status: 404 });
		const parts = url.pathname.split('/').filter(Boolean);
		if (parts.length !== 2) return new Response('Not found', { status: 404 });
		const entry = dependencies.registry.resolve(parts[0]);
		if (!entry) return new Response('PDF preview token expired', { status: 404 });
		try {
			if (decodeURIComponent(parts[1]) !== entry.publicName)
				return new Response('Not found', { status: 404 });
		} catch {
			return new Response('Not found', { status: 404 });
		}
		let upstream: Response;
		try {
			const currentPath = await dependencies.fs.promises.realpath(entry.absolutePath);
			if (
				currentPath !== entry.absolutePath ||
				!withinRoot(entry.root, currentPath, dependencies.path)
			) {
				return new Response('PDF preview file changed', { status: 404 });
			}
			const stat = await dependencies.fs.promises.stat(currentPath);
			if (!stat.isFile() || stat.size > PDF_PREVIEW_MAX_BYTES) {
				return new Response('PDF preview file unavailable', { status: 404 });
			}
			upstream = await dependencies.net.fetch(pathToFileURL(currentPath).toString(), {
				method: request.method,
				headers: request.headers
			});
		} catch {
			return new Response('PDF preview file unavailable', { status: 404 });
		}
		if (!upstream.ok && upstream.status !== 206) {
			return new Response('Failed to load PDF preview', { status: upstream.status || 500 });
		}
		const headers = new Headers(upstream.headers);
		headers.set('content-type', 'application/pdf');
		headers.set(
			'content-disposition',
			`inline; filename="${entry.publicName.replace(/["\\]/g, '_')}"`
		);
		return new Response(request.method === 'HEAD' ? null : upstream.body, {
			status: upstream.status,
			statusText: upstream.statusText,
			headers
		});
	});
}
