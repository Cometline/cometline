type FileSystem = Pick<typeof import('node:fs'), 'promises'>;
type PathService = Pick<typeof import('node:path'), 'extname' | 'resolve' | 'sep'>;

export type WorkspaceFilePreviewResult =
	| { ok: true; kind: 'text'; content: string; extension: string }
	| { ok: true; kind: 'image'; mimeType: string; dataUrl: string }
	| { ok: false; error: string };

export interface WorkspacePreviewDependencies {
	fs: FileSystem;
	path: PathService;
}

const WORKSPACE_FILE_MAX_BYTES = 256 * 1024;
const IMAGE_MIME_BY_EXT: Record<string, string> = {
	'.png': 'image/png',
	'.jpg': 'image/jpeg',
	'.jpeg': 'image/jpeg',
	'.gif': 'image/gif',
	'.webp': 'image/webp',
	'.svg': 'image/svg+xml'
};

/** Limits renderer previews to regular files inside the selected workspace. */
export async function readWorkspaceFileForPreview(
	dependencies: WorkspacePreviewDependencies,
	workspacePath: unknown,
	relativePath: unknown
): Promise<WorkspaceFilePreviewResult> {
	const root = dependencies.path.resolve(String(workspacePath || ''));
	const clean = String(relativePath || '').replace(/^[/\\]+/, '');
	if (!root || root === dependencies.path.sep || !clean) {
		return { ok: false, error: 'Invalid file path' };
	}

	const absolutePath = dependencies.path.resolve(root, clean);
	if (absolutePath !== root && !absolutePath.startsWith(root + dependencies.path.sep)) {
		return { ok: false, error: 'Path escapes workspace' };
	}

	let stat: Awaited<ReturnType<FileSystem['promises']['stat']>>;
	try {
		stat = await dependencies.fs.promises.stat(absolutePath);
	} catch {
		return { ok: false, error: 'File not found' };
	}
	if (!stat.isFile()) return { ok: false, error: 'Not a file' };
	if (stat.size > WORKSPACE_FILE_MAX_BYTES) {
		return { ok: false, error: 'File exceeds 256 KB preview limit' };
	}

	const extension = dependencies.path.extname(absolutePath).toLowerCase();
	const mimeType = IMAGE_MIME_BY_EXT[extension];
	if (mimeType) {
		const buffer = await dependencies.fs.promises.readFile(absolutePath);
		return {
			ok: true,
			kind: 'image',
			mimeType,
			dataUrl: `data:${mimeType};base64,${buffer.toString('base64')}`
		};
	}

	let content: string;
	try {
		content = await dependencies.fs.promises.readFile(absolutePath, 'utf8');
	} catch {
		return { ok: false, error: 'Cannot read file as text' };
	}
	if (content.includes('\0')) return { ok: false, error: 'Binary file cannot be previewed' };

	return { ok: true, kind: 'text', content, extension };
}

/** Permits browser and mail links only; native local handlers remain inaccessible to the renderer. */
export function isExternallyOpenableUrl(rawUrl: unknown) {
	try {
		const parsed = new URL(String(rawUrl));
		return ['http:', 'https:', 'mailto:'].includes(parsed.protocol);
	} catch {
		return false;
	}
}
