import type fs from 'node:fs';
import type path from 'node:path';

const DEBOUNCE_MS = 300;
const SKIPPED_DIRECTORIES = new Set([
	'node_modules',
	'vendor',
	'dist',
	'build',
	'.next',
	'out',
	'coverage',
	'__pycache__'
]);

export type WorkspaceChange = {
	workspacePath: string;
	paths: string[];
	gitChanged: boolean;
};

type WorkspaceWatcherDependencies = {
	fs: Pick<typeof fs, 'statSync' | 'watch'>;
	path: Pick<typeof path, 'resolve'>;
	onChange(change: WorkspaceChange): void;
	setTimeout: typeof setTimeout;
	clearTimeout: typeof clearTimeout;
};

function normalizeRelativePath(filePath: string): string {
	return filePath.replace(/\\/g, '/').replace(/^\.\//, '').replace(/^\/+/, '');
}

function isGitMetadata(filePath: string): boolean {
	return filePath === '.git' || filePath.startsWith('.git/');
}

function shouldSkip(filePath: string): boolean {
	return filePath.split('/').some((segment) => SKIPPED_DIRECTORIES.has(segment));
}

/** Watches one active workspace and coalesces filesystem noise into useful UI refreshes. */
export function createWorkspaceWatcher(dependencies: WorkspaceWatcherDependencies) {
	let watcher: fs.FSWatcher | null = null;
	let workspacePath = '';
	const changedPaths = new Set<string>();
	let gitChanged = false;
	let flushTimer: ReturnType<typeof setTimeout> | null = null;

	function flush() {
		flushTimer = null;
		if (!workspacePath) return;
		const paths = [...changedPaths].sort();
		changedPaths.clear();
		const changedGit = gitChanged;
		gitChanged = false;
		if (paths.length === 0 && !changedGit) return;
		dependencies.onChange({ workspacePath, paths, gitChanged: changedGit });
	}

	function scheduleFlush() {
		if (flushTimer) dependencies.clearTimeout(flushTimer);
		flushTimer = dependencies.setTimeout(flush, DEBOUNCE_MS);
	}

	function record(filename: string | Buffer | null) {
		if (!workspacePath) return;
		if (filename === null) {
			gitChanged = true;
			scheduleFlush();
			return;
		}
		const relativePath = normalizeRelativePath(String(filename));
		if (!relativePath) {
			gitChanged = true;
			scheduleFlush();
			return;
		}
		if (isGitMetadata(relativePath)) {
			gitChanged = true;
			scheduleFlush();
			return;
		}
		if (shouldSkip(relativePath)) return;
		changedPaths.add(relativePath);
		scheduleFlush();
	}

	function close() {
		watcher?.close();
		watcher = null;
		workspacePath = '';
		changedPaths.clear();
		gitChanged = false;
		if (flushTimer) dependencies.clearTimeout(flushTimer);
		flushTimer = null;
	}

	function watch(nextWorkspacePath: string) {
		const next = dependencies.path.resolve(String(nextWorkspacePath || '').trim());
		if (!next || next === workspacePath) return;
		close();

		try {
			if (!dependencies.fs.statSync(next).isDirectory()) return;
			workspacePath = next;
			watcher = dependencies.fs.watch(next, { recursive: true }, (_eventType, filename) => {
				if (workspacePath !== next) return;
				record(filename);
			});
			watcher.on('error', () => {
				// A future focus refresh recovers the UI if an OS watcher is invalidated.
				gitChanged = true;
				scheduleFlush();
			});
		} catch {
			close();
		}
	}

	return { close, watch };
}
