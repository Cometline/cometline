import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { createMediaClipboard } from './media-clipboard.js';

const temporaryDirectories: string[] = [];

afterEach(() => {
	for (const directory of temporaryDirectories.splice(0)) {
		fs.rmSync(directory, { recursive: true, force: true });
	}
});

function fixture(platform: NodeJS.Platform = 'darwin') {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-media-clipboard-'));
	const home = path.join(root, "home with ' quote");
	const sessionDirectory = path.join(home, '.cometmind', 'media', 'session-1');
	fs.mkdirSync(sessionDirectory, { recursive: true });
	fs.writeFileSync(path.join(sessionDirectory, 'video-1.mp4'), 'video');
	temporaryDirectories.push(root);
	const run = vi.fn().mockResolvedValue(undefined);
	const clipboard = createMediaClipboard({ fs, path, homedir: () => home, platform, run });
	return { clipboard, home, root, run };
}

describe('media clipboard', () => {
	it('copies a registered video path through an AppleScript argument', async () => {
		const { clipboard, home, run } = fixture();

		await expect(clipboard.copyMediaFile('session-1', 'video-1')).resolves.toEqual({ ok: true });
		expect(run).toHaveBeenCalledWith('osascript', [
			'-e',
			'on run argv',
			'-e',
			'set the clipboard to POSIX file (item 1 of argv)',
			'-e',
			'end run',
			fs.realpathSync(path.join(home, '.cometmind', 'media', 'session-1', 'video-1.mp4'))
		]);
	});

	it('rejects traversal and arbitrary path input', async () => {
		const { clipboard, run } = fixture();

		await expect(clipboard.copyMediaFile('../outside', 'video-1')).resolves.toEqual({
			ok: false,
			error: 'Invalid media reference.'
		});
		await expect(clipboard.copyMediaFile('session-1', '../../outside')).resolves.toEqual({
			ok: false,
			error: 'Invalid media reference.'
		});
		expect(run).not.toHaveBeenCalled();
	});

	it('reports deleted or unknown videos without invoking AppleScript', async () => {
		const { clipboard, run } = fixture();

		await expect(clipboard.copyMediaFile('session-1', 'missing')).resolves.toEqual({
			ok: false,
			error: 'Video file was not found.'
		});
		expect(run).not.toHaveBeenCalled();
	});

	it('rejects media files reached through a session symlink outside the media root', async () => {
		const { clipboard, home, root, run } = fixture();
		const sessionDirectory = path.join(home, '.cometmind', 'media', 'session-1');
		const outsideDirectory = path.join(root, 'outside');
		fs.rmSync(sessionDirectory, { recursive: true });
		fs.mkdirSync(outsideDirectory);
		fs.writeFileSync(path.join(outsideDirectory, 'video-1.mp4'), 'video');
		fs.symlinkSync(outsideDirectory, sessionDirectory);

		await expect(clipboard.copyMediaFile('session-1', 'video-1')).resolves.toEqual({
			ok: false,
			error: 'Invalid media reference.'
		});
		expect(run).not.toHaveBeenCalled();
	});

	it('does not attempt native copy on unsupported platforms', async () => {
		const { clipboard, run } = fixture('win32');

		await expect(clipboard.copyMediaFile('session-1', 'video-1')).resolves.toEqual({
			ok: false,
			error: 'Copying video files is currently available on macOS only.'
		});
		expect(run).not.toHaveBeenCalled();
	});
});
