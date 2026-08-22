import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const mocks = vi.hoisted(() => ({
	setSidebarOrderWorkspacePath: vi.fn(),
	setSidebarOrderDiscordActive: vi.fn()
}));

vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		sidebarOrderWorkspacePath: '/ws-a',
		setSidebarOrderWorkspacePath: mocks.setSidebarOrderWorkspacePath,
		setSidebarOrderDiscordActive: mocks.setSidebarOrderDiscordActive
	}
}));

import { commitSidebarWorkspaceForSession } from './commit-sidebar-workspace';

function session(overrides: Partial<Session> = {}): Session {
	return {
		id: 'sess-1',
		workspace_id: 'ws-1',
		workspace_path: '/ws-b',
		title: 'Test',
		model_id: 'm',
		provider_id: 'p',
		status: 'active',
		origin: 'user',
		token_usage: {
			input_tokens: 0,
			output_tokens: 0,
			cache_read: 0,
			cache_write: 0
		},
		pinned: false,
			agent_mode: 'auto',
		running: false,
		created_at: 0,
		updated_at: 0,
		...overrides
	};
}

describe('commitSidebarWorkspaceForSession', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('commits workspace and discord flag for unpinned sessions', () => {
		commitSidebarWorkspaceForSession(session());

		expect(mocks.setSidebarOrderWorkspacePath).toHaveBeenCalledWith('/ws-b');
		expect(mocks.setSidebarOrderDiscordActive).toHaveBeenCalledWith(false);
	});

	it('does not commit sidebar order for pinned sessions', () => {
		commitSidebarWorkspaceForSession(session({ pinned: true }));

		expect(mocks.setSidebarOrderWorkspacePath).not.toHaveBeenCalled();
		expect(mocks.setSidebarOrderDiscordActive).not.toHaveBeenCalled();
	});

	it('sets discord active for unpinned discord sessions', () => {
		commitSidebarWorkspaceForSession(
			session({
				gateway: { platform: 'discord', channel_id: '123', thread_id: '' }
			})
		);

		expect(mocks.setSidebarOrderDiscordActive).toHaveBeenCalledWith(true);
	});
});
