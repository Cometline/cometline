import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/environment', () => ({ browser: false }));

const STORAGE_KEY = 'cometline.unread-session-output.v1';

function installLocalStorageMock() {
	const store = new Map<string, string>();
	Object.defineProperty(globalThis, 'localStorage', {
		configurable: true,
		value: {
			getItem: (key: string) => store.get(key) ?? null,
			setItem: (key: string, value: string) => store.set(key, value),
			removeItem: (key: string) => store.delete(key),
			clear: () => store.clear()
		}
	});
}

describe('unreadSessionOutputStore', () => {
	beforeEach(() => {
		installLocalStorageMock();
		localStorage.clear();
		vi.resetModules();
	});

	afterEach(() => {
		localStorage.clear();
	});

	it('persists unread session IDs and clears them when read', async () => {
		const { unreadSessionOutputStore } = await import('./unread-session-output.svelte');

		unreadSessionOutputStore.markUnread('session-a');

		expect(unreadSessionOutputStore.isUnread('session-a')).toBe(true);
		expect(localStorage.getItem(STORAGE_KEY)).toBe(JSON.stringify(['session-a']));

		unreadSessionOutputStore.markRead('session-a');

		expect(unreadSessionOutputStore.isUnread('session-a')).toBe(false);
		expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
	});

	it('restores valid IDs and ignores malformed persisted values', async () => {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(['session-a', '', 42]));
		let module = await import('./unread-session-output.svelte');

		expect(module.unreadSessionOutputStore.isUnread('session-a')).toBe(true);
		expect(module.unreadSessionOutputStore.isUnread('42')).toBe(false);

		vi.resetModules();
		localStorage.setItem(STORAGE_KEY, '{not json');
		module = await import('./unread-session-output.svelte');

		expect(module.unreadSessionOutputStore.unreadSessionIds.size).toBe(0);
	});

	it('prunes IDs for deleted sessions', async () => {
		const { unreadSessionOutputStore } = await import('./unread-session-output.svelte');
		unreadSessionOutputStore.markUnread('session-a');
		unreadSessionOutputStore.markUnread('session-b');

		unreadSessionOutputStore.prune(['session-b']);

		expect(unreadSessionOutputStore.isUnread('session-a')).toBe(false);
		expect(unreadSessionOutputStore.isUnread('session-b')).toBe(true);
	});
});
