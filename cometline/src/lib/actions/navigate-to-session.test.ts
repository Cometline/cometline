import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const mocks = vi.hoisted(() => ({
	goto: vi.fn().mockResolvedValue(undefined),
	selectSession: vi.fn(),
	selectFromSession: vi.fn(),
	setActiveWorkspacePath: vi.fn(),
	setSidebarOrderWorkspacePath: vi.fn(),
	setSidebarOrderDiscordActive: vi.fn(),
	recordVisit: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/stores/session.svelte', () => ({
	sessionStore: { selectSession: mocks.selectSession }
}));
vi.mock('$lib/stores/model.svelte', () => ({
	modelStore: { selectFromSession: mocks.selectFromSession }
}));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		workspacePath: '/ws-a',
		setActiveWorkspacePath: mocks.setActiveWorkspacePath,
		setSidebarOrderWorkspacePath: mocks.setSidebarOrderWorkspacePath,
		setSidebarOrderDiscordActive: mocks.setSidebarOrderDiscordActive
	}
}));
vi.mock('$lib/stores/session-visit-history.svelte', () => ({
	sessionVisitHistory: { recordVisit: mocks.recordVisit }
}));

import { navigateToSession } from './navigate-to-session';

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
		created_at: 0,
		updated_at: 0,
		...overrides
	};
}

describe('navigateToSession sidebar order', () => {
	const electronSetWorkspacePath = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		vi.stubGlobal('window', { electronAPI: { setWorkspacePath: electronSetWorkspacePath } });
	});

	it('commits sidebar order for unpinned sessions by default', () => {
		navigateToSession(session());

		expect(mocks.setSidebarOrderWorkspacePath).toHaveBeenCalledWith('/ws-b');
		expect(mocks.setSidebarOrderDiscordActive).toHaveBeenCalledWith(false);
	});

	it('does not commit sidebar order for pinned sessions by default', () => {
		navigateToSession(session({ pinned: true }));

		expect(mocks.setSidebarOrderWorkspacePath).not.toHaveBeenCalled();
		expect(mocks.setSidebarOrderDiscordActive).not.toHaveBeenCalled();
	});

	it('allows explicit commitSidebarOrder override for pinned sessions', () => {
		navigateToSession(session({ pinned: true }), { commitSidebarOrder: true });

		expect(mocks.setSidebarOrderWorkspacePath).toHaveBeenCalledWith('/ws-b');
		expect(mocks.setSidebarOrderDiscordActive).toHaveBeenCalledWith(false);
	});

	it('still opens the session and updates active workspace when pinned', () => {
		navigateToSession(session({ pinned: true }));

		expect(mocks.selectSession).toHaveBeenCalled();
		expect(mocks.selectFromSession).toHaveBeenCalled();
		expect(mocks.setActiveWorkspacePath).toHaveBeenCalledWith('/ws-b');
		expect(electronSetWorkspacePath).not.toHaveBeenCalled();
		expect(mocks.goto).toHaveBeenCalledWith('/session/sess-1');
	});

	it('does not persist workspace to Electron when switching sessions', () => {
		navigateToSession(session());

		expect(mocks.setActiveWorkspacePath).toHaveBeenCalledWith('/ws-b');
		expect(electronSetWorkspacePath).not.toHaveBeenCalled();
	});

	it('records a visit for normal navigation', () => {
		navigateToSession(session());
		expect(mocks.recordVisit).toHaveBeenCalledWith('sess-1');
	});

	it('skips visit recording when navigating from history', () => {
		navigateToSession(session(), { fromHistory: true });
		expect(mocks.recordVisit).not.toHaveBeenCalled();
	});
});
