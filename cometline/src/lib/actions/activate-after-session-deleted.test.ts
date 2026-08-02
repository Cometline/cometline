import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const mocks = vi.hoisted(() => ({
	startNewChat: vi.fn(),
	navigateToSession: vi.fn(),
	clear: vi.fn(),
	sessions: [] as Session[],
	sidebarOrderWorkspacePath: '/ws',
	sidebarOrderDiscordActive: false
}));

vi.mock('$lib/actions/new-chat', () => ({ startNewChat: mocks.startNewChat }));
vi.mock('$lib/actions/navigate-to-session', () => ({
	navigateToSession: mocks.navigateToSession
}));
vi.mock('$lib/stores/chat.svelte', () => ({ chatStore: { clear: mocks.clear } }));
vi.mock('$lib/stores/session.svelte', () => ({
	sessionStore: {
		get sessions() {
			return mocks.sessions;
		}
	}
}));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		get sidebarOrderWorkspacePath() {
			return mocks.sidebarOrderWorkspacePath;
		},
		get sidebarOrderDiscordActive() {
			return mocks.sidebarOrderDiscordActive;
		}
	}
}));

import { activateAfterSessionDeleted } from './activate-after-session-deleted';

function session(id: string, workspacePath = '/ws'): Session {
	return {
		id,
		workspace_id: 'ws',
		workspace_path: workspacePath,
		title: id,
		model_id: 'm',
		provider_id: 'p',
		status: 'active',
		origin: 'user',
		token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
		pinned: false,
			agent_mode: 'auto',
		created_at: 0,
		updated_at: id === 'b' ? 2 : 1
	};
}

describe('activateAfterSessionDeleted', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.startNewChat.mockResolvedValue(undefined);
	});

	it('navigates to the next existing session', async () => {
		const before = [session('a'), session('b'), session('c')];
		before[0] = { ...before[0], updated_at: 3 };
		before[1] = { ...before[1], updated_at: 2 };
		before[2] = { ...before[2], updated_at: 1 };
		mocks.sessions = [
			{ ...session('b'), updated_at: 2 },
			{ ...session('c'), updated_at: 1 }
		];
		await activateAfterSessionDeleted('a', before);
		expect(mocks.clear).toHaveBeenCalledOnce();
		expect(mocks.navigateToSession).toHaveBeenCalledWith(expect.objectContaining({ id: 'b' }));
		expect(mocks.startNewChat).not.toHaveBeenCalled();
	});

	it('skips neighbors that were also removed', async () => {
		const before = [session('a'), session('b'), session('c')];
		mocks.sessions = [session('c')];
		await activateAfterSessionDeleted('a', before);
		expect(mocks.navigateToSession).toHaveBeenCalledWith(expect.objectContaining({ id: 'c' }));
	});

	it('starts a new chat when no sessions remain', async () => {
		mocks.sessions = [];
		await activateAfterSessionDeleted('only', [session('only')]);
		expect(mocks.navigateToSession).not.toHaveBeenCalled();
		expect(mocks.startNewChat).toHaveBeenCalledOnce();
	});
});
