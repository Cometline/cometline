import { execFile } from 'node:child_process';
import type fs from 'node:fs';
import type path from 'node:path';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const VIDEO_EXTENSIONS = new Set(['.mp4', '.webm']);

type FileSystem = Pick<typeof fs, 'promises'>;
type PathService = Pick<typeof path, 'extname' | 'join' | 'resolve' | 'sep'>;

export type CopyMediaFileResult = { ok: true } | { ok: false; error: string };

export interface MediaClipboardDependencies {
	fs: FileSystem;
	path: PathService;
	homedir(): string;
	platform: NodeJS.Platform;
	run?: (command: string, args: string[]) => Promise<unknown>;
}

function safeSegment(value: unknown): string | null {
	const segment = typeof value === 'string' ? value.trim() : '';
	if (!segment || segment.includes('..') || /[/\\\0]/.test(segment)) return null;
	return segment;
}

function withinRoot(root: string, target: string, pathService: PathService): boolean {
	return target === root || target.startsWith(`${root}${pathService.sep}`);
}

/** Copies a registered CometMind video as a file without accepting renderer-supplied paths. */
export function createMediaClipboard(dependencies: MediaClipboardDependencies) {
	const run = dependencies.run ?? execFileAsync;

	async function copyMediaFile(sessionIdInput: unknown, mediaIdInput: unknown): Promise<CopyMediaFileResult> {
		if (dependencies.platform !== 'darwin') {
			return { ok: false, error: 'Copying video files is currently available on macOS only.' };
		}

		const sessionId = safeSegment(sessionIdInput);
		const mediaId = safeSegment(mediaIdInput);
		if (!sessionId || !mediaId) return { ok: false, error: 'Invalid media reference.' };

		try {
			const mediaRoot = await dependencies.fs.promises.realpath(
				dependencies.path.resolve(dependencies.homedir(), '.cometmind', 'media')
			);
			const sessionDirectory = dependencies.path.resolve(mediaRoot, sessionId);
			if (!withinRoot(mediaRoot, sessionDirectory, dependencies.path)) {
				return { ok: false, error: 'Invalid media reference.' };
			}

			const entries = await dependencies.fs.promises.readdir(sessionDirectory, {
				withFileTypes: true
			});
			const entry = entries.find((candidate) => {
				if (!candidate.isFile()) return false;
				const extension = dependencies.path.extname(candidate.name).toLowerCase();
				return VIDEO_EXTENSIONS.has(extension) && candidate.name === `${mediaId}${extension}`;
			});
			if (!entry) return { ok: false, error: 'Video file was not found.' };

			const filePath = await dependencies.fs.promises.realpath(
				dependencies.path.join(sessionDirectory, entry.name)
			);
			if (!withinRoot(mediaRoot, filePath, dependencies.path)) {
				return { ok: false, error: 'Invalid media reference.' };
			}

			await run('osascript', [
				'-e',
				'on run argv',
				'-e',
				'set the clipboard to POSIX file (item 1 of argv)',
				'-e',
				'end run',
				filePath
			]);
			return { ok: true };
		} catch (error) {
			if ((error as NodeJS.ErrnoException)?.code === 'ENOENT') {
				return { ok: false, error: 'Video file was not found.' };
			}
			return { ok: false, error: 'Could not copy this video.' };
		}
	}

	return { copyMediaFile };
}
