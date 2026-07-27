import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWorkspaceWatcher, type WorkspaceChange } from './workspace-watcher.js';

type Listener = (eventType: string, filename: string | Buffer | null) => void;

describe('workspace watcher', () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it('coalesces workspace files and Git metadata into one change event', () => {
		vi.useFakeTimers();
		const changes: WorkspaceChange[] = [];
		let emit: Listener = () => {};
		const watcher = createWorkspaceWatcher({
			fs: {
				statSync: () => ({ isDirectory: () => true }),
				watch: (_path: unknown, _options: unknown, callback: unknown) => {
					emit = callback as Listener;
					return { close: vi.fn(), on: vi.fn() };
				}
			} as never,
			path: { resolve: (value: string) => value },
			onChange: (change) => changes.push(change),
			setTimeout,
			clearTimeout
		});

		watcher.watch('/workspace');
		emit('change', 'src/app.ts');
		emit('rename', Buffer.from('README.md'));
		emit('change', '.git/index');
		vi.advanceTimersByTime(300);

		expect(changes).toEqual([
			{ workspacePath: '/workspace', paths: ['README.md', 'src/app.ts'], gitChanged: true }
		]);
	});

	it('ignores generated output and closes the prior watcher on workspace switch', () => {
		vi.useFakeTimers();
		const changes: WorkspaceChange[] = [];
		const close = vi.fn();
		const listeners: Listener[] = [];
		const watcher = createWorkspaceWatcher({
			fs: {
				statSync: () => ({ isDirectory: () => true }),
				watch: (_path: unknown, _options: unknown, callback: unknown) => {
					listeners.push(callback as Listener);
					return { close, on: vi.fn() };
				}
			} as never,
			path: { resolve: (value: string) => value },
			onChange: (change) => changes.push(change),
			setTimeout,
			clearTimeout
		});

		watcher.watch('/first');
		listeners[0]?.('change', 'node_modules/library/index.js');
		vi.advanceTimersByTime(300);
		expect(changes).toEqual([]);

		watcher.watch('/second');
		expect(close).toHaveBeenCalledTimes(1);
		listeners[0]?.('change', 'src/stale.ts');
		listeners[1]?.('change', 'src/new.ts');
		vi.advanceTimersByTime(300);
		expect(changes).toEqual([
			{ workspacePath: '/second', paths: ['src/new.ts'], gitChanged: false }
		]);
	});
});
