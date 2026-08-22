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
	recordVisit: vi.fn(),
	requestNewSession: vi.fn(),
	clearNewSessionRequest: vi.fn(),
	startOpening: vi.fn(),
	resetOpening: vi.fn(),
	sessionState: {
		sessions: [] as Session[],
		loaded: false
	}
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
	sessionStore: {
		selectSession: mocks.selectSession,
		upsertSession: vi.fn(),
		appendSession: vi.fn(),
		get sessions() {
			return mocks.sessionState.sessions;
		},
		get loaded() {
			return mocks.sessionState.loaded;
		}
	}
}));
vi.mock('$lib/stores/settings.svelte', () => ({ settingsStore: { load: vi.fn() } }));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		workspacePath: '/current-workspace',
		setActiveWorkspacePath: mocks.setActiveWorkspacePath,
		setSidebarOrderWorkspacePath: mocks.setSidebarOrderWorkspacePath,
		setSidebarOrderDiscordActive: mocks.setSidebarOrderDiscordActive,
		requestComposerFocus: mocks.requestComposerFocus,
	}
}));
vi.mock('$lib/stores/session-visit-history.svelte', () => ({
	sessionVisitHistory: { recordVisit: mocks.recordVisit }
}));
vi.mock('$lib/stores/mini-shell.svelte', () => ({
	miniShellStore: {
		requestNewSession: mocks.requestNewSession,
		clearNewSessionRequest: mocks.clearNewSessionRequest,
		startOpening: mocks.startOpening,
		resetOpening: mocks.resetOpening
	}
}));

import {
	activateMiniWindow,
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
	running: false,
	created_at: 0,
	updated_at: 0
};

describe('mini window sessions', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.sessionState.loaded = false;
		mocks.sessionState.sessions = [];
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
		expect(mocks.goto.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.requestComposerFocus.mock.invocationCallOrder[0]
		);
	});

	it('creates a default-model session and opens it in the mini route', async () => {
		await expect(createMiniWindowSession()).resolves.toEqual(session);

		expect(mocks.startOpening).toHaveBeenCalledOnce();
		expect(mocks.startOpening.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.createNewSession.mock.invocationCallOrder[0]
		);
		expect(mocks.createNewSession).toHaveBeenCalledOnce();
		expect(window.electronAPI?.saveMiniWindowState).toHaveBeenCalledWith({
			sessionId: 'mini-session'
		});
		expect(mocks.requestNewSession).toHaveBeenCalledWith('mini-session');
		expect(mocks.requestComposerFocus).toHaveBeenCalledWith('mini-session');
		expect(mocks.goto).toHaveBeenCalledWith('/mini/session/mini-session');
		expect(mocks.goto.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.requestComposerFocus.mock.invocationCallOrder[0]
		);
	});

	it('clears the matching loading request when navigation fails', async () => {
		mocks.goto.mockRejectedValueOnce(new Error('navigation failed'));

		await expect(createMiniWindowSession()).rejects.toThrow('navigation failed');

		expect(mocks.clearNewSessionRequest).toHaveBeenCalledWith('mini-session');
		expect(mocks.resetOpening).toHaveBeenCalledOnce();
	});

	it('reuses a preferred session from another workspace', async () => {
		await expect(ensureMiniWindowSession('mini-session')).resolves.toBe('mini-session');

		expect(mocks.listAllSessions).toHaveBeenCalledOnce();
		expect(mocks.createNewSession).not.toHaveBeenCalled();
	});

	it('does not create a replacement for a missing preferred session', async () => {
		mocks.listAllSessions.mockResolvedValue({ sessions: [] });

		await expect(ensureMiniWindowSession('missing-session')).rejects.toThrow(
			'This chat no longer exists.'
		);

		expect(mocks.createNewSession).not.toHaveBeenCalled();
	});

	it('reuses a loaded session without listing from the API', async () => {
		mocks.sessionState.loaded = true;
		mocks.sessionState.sessions = [session];
		vi.stubGlobal('window', {
			electronAPI: {
				getMiniWindowState: vi.fn().mockResolvedValue({
					sessionId: 'mini-session',
					lastActiveAt: Date.now(),
					inactivityTimeoutMinutes: 30
				}),
				saveMiniWindowState: vi.fn().mockResolvedValue(undefined)
			}
		});

		await expect(ensureMiniWindowSession()).resolves.toBe('mini-session');
		expect(mocks.listAllSessions).not.toHaveBeenCalled();
	});

	it('shares one in-flight activation across repeated shortcut shows', async () => {
		let resolveList: ((value: { sessions: Session[] }) => void) | undefined;
		mocks.listAllSessions.mockReturnValue(
			new Promise<{ sessions: Session[] }>((resolve) => {
				resolveList = resolve;
			})
		);
		vi.stubGlobal('window', {
			electronAPI: {
				getMiniWindowState: vi.fn().mockResolvedValue({
					sessionId: 'mini-session',
					lastActiveAt: Date.now(),
					inactivityTimeoutMinutes: 30
				}),
				getWorkspacePath: vi.fn().mockResolvedValue('/'),
				saveMiniWindowState: vi.fn().mockResolvedValue(undefined)
			}
		});

		const first = activateMiniWindow();
		const second = activateMiniWindow();
		resolveList?.({ sessions: [session] });

		await expect(Promise.all([first, second])).resolves.toEqual([
			'mini-session',
			'mini-session'
		]);
		expect(mocks.goto).toHaveBeenCalledOnce();
		expect(mocks.goto).toHaveBeenCalledWith('/mini/session/mini-session', {
			replaceState: true
		});
	});
});
