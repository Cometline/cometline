import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const mocks = vi.hoisted(() => ({
	goto: vi.fn().mockResolvedValue(undefined),
	createNewSession: vi.fn(),
	listAllSessions: vi.fn(),
	selectSession: vi.fn(),
	selectFromSession: vi.fn(),
	setActiveWorkspacePath: vi.fn(),
	setSidebarOrderWorkspacePath: vi.fn(),
	setSidebarOrderDiscordActive: vi.fn(),
	requestComposerFocus: vi.fn(),
	recordVisit: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/actions/create-new-session', () => ({ createNewSession: mocks.createNewSession }));
vi.mock('$lib/client/cometmind', () => ({
	createSession: vi.fn(),
	listAllSessions: mocks.listAllSessions
}));
vi.mock('$lib/stores/model.svelte', () => ({
	modelStore: { options: [], selected: null, selectDefault: vi.fn(), selectFromSession: mocks.selectFromSession }
}));
vi.mock('$lib/stores/session.svelte', () => ({
	sessionStore: { selectSession: mocks.selectSession, upsertSession: vi.fn(), appendSession: vi.fn() }
}));
vi.mock('$lib/stores/settings.svelte', () => ({ settingsStore: { load: vi.fn() } }));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		workspacePath: '/current-workspace',
		setActiveWorkspacePath: mocks.setActiveWorkspacePath,
		setSidebarOrderWorkspacePath: mocks.setSidebarOrderWorkspacePath,
		setSidebarOrderDiscordActive: mocks.setSidebarOrderDiscordActive,
		requestComposerFocus: mocks.requestComposerFocus
	}
}));
vi.mock('$lib/stores/session-visit-history.svelte', () => ({
	sessionVisitHistory: { recordVisit: mocks.recordVisit }
}));

import {
	createMiniWindowSession,
	ensureMiniWindowSession,
	navigateMiniToSession
} from './mini-window-session';

const session: Session = {
	id: 'mini-session',
	workspace_id: 'workspace-2',
	workspace_path: '/other-workspace',
	title: '',
	model_id: 'model',
	provider_id: 'provider',
	status: 'active',
	origin: 'user',
	token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
	pinned: false,
	agent_mode: 'auto',
	created_at: 0,
	updated_at: 0
};

describe('mini window sessions', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.createNewSession.mockResolvedValue(session);
		mocks.listAllSessions.mockResolvedValue({ sessions: [session] });
		vi.stubGlobal('window', {
			electronAPI: { saveMiniWindowState: vi.fn().mockResolvedValue(undefined) }
		});
	});

	it('keeps selected session navigation inside the mini route namespace', async () => {
		await navigateMiniToSession(session);

		expect(mocks.selectSession).toHaveBeenCalledWith(session);
		expect(mocks.selectFromSession).toHaveBeenCalledWith(session);
		expect(mocks.setActiveWorkspacePath).toHaveBeenCalledWith('/other-workspace');
		expect(mocks.setSidebarOrderWorkspacePath).toHaveBeenCalledWith('/other-workspace');
		expect(mocks.recordVisit).toHaveBeenCalledWith('mini-session');
		expect(window.electronAPI?.saveMiniWindowState).toHaveBeenCalledWith({
			sessionId: 'mini-session'
		});
		expect(mocks.goto).toHaveBeenCalledWith('/mini/session/mini-session');
		expect(mocks.requestComposerFocus).toHaveBeenCalledWith('mini-session');
		expect(mocks.requestComposerFocus.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.goto.mock.invocationCallOrder[0]
		);
	});

	it('creates a default-model session and opens it in the mini route', async () => {
		await expect(createMiniWindowSession()).resolves.toEqual(session);

		expect(mocks.createNewSession).toHaveBeenCalledOnce();
		expect(window.electronAPI?.saveMiniWindowState).toHaveBeenCalledWith({
			sessionId: 'mini-session'
		});
		expect(mocks.goto).toHaveBeenCalledWith('/mini/session/mini-session');
		expect(mocks.requestComposerFocus).toHaveBeenCalledWith('mini-session');
		expect(mocks.requestComposerFocus.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.goto.mock.invocationCallOrder[0]
		);
	});

	it('reuses a preferred session from another workspace', async () => {
		await expect(ensureMiniWindowSession('mini-session')).resolves.toBe('mini-session');

		expect(mocks.listAllSessions).toHaveBeenCalledOnce();
		expect(mocks.createNewSession).not.toHaveBeenCalled();
	});
});
