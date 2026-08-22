import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';
import { bootstrapHomeSession, type BootstrapHomeSessionDeps } from './bootstrap-home-session';

describe('bootstrapHomeSession', () => {
	const startNewChat = vi.fn();
	const navigateToSession = vi.fn();
	const mostRecentSessionId = vi.fn();

	function session(id: string, updatedAt: number): Session {
		return {
			id,
			workspace_id: 'workspace-1',
			workspace_path: '/ws',
			title: id,
			model_id: 'model',
			provider_id: 'provider',
			status: 'active',
			origin: 'user',
			token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
			pinned: false,
			agent_mode: 'auto',
			running: false,
			created_at: 0,
			updated_at: updatedAt
		};
	}

	function deps(overrides: Partial<BootstrapHomeSessionDeps> = {}): BootstrapHomeSessionDeps {
		return {
			connectionStatus: () => 'ready',
			workspacePath: () => '/ws',
			sessionsLoaded: () => true,
			sessions: () => [],
			mostRecentSessionId,
			navigateToSession,
			startNewChat,
			...overrides
		};
	}

	beforeEach(() => {
		vi.clearAllMocks();
		startNewChat.mockResolvedValue(undefined);
		navigateToSession.mockResolvedValue(undefined);
		mostRecentSessionId.mockReturnValue(null);
	});

	it('skips when the sidecar is not ready', async () => {
		await expect(
			bootstrapHomeSession(deps({
				connectionStatus: () => 'connecting',
			}))
		).resolves.toBe(false);
		expect(startNewChat).not.toHaveBeenCalled();
	});

	it('skips when workspace is unset', async () => {
		await expect(
			bootstrapHomeSession(deps({
				workspacePath: () => '/',
			}))
		).resolves.toBe(false);
		expect(startNewChat).not.toHaveBeenCalled();
	});

	it('waits for the session list before choosing a session', async () => {
		await expect(
			bootstrapHomeSession(deps({ sessionsLoaded: () => false }))
		).resolves.toBe(false);
		expect(startNewChat).not.toHaveBeenCalled();
		expect(navigateToSession).not.toHaveBeenCalled();
	});

	it('creates a session when no sessions exist', async () => {
		await expect(bootstrapHomeSession(deps())).resolves.toBe(true);
		expect(startNewChat).toHaveBeenCalledOnce();
	});

	it('restores the persisted most recently visited session', async () => {
		const sessions = [session('older', 1), session('last-visited', 2)];
		mostRecentSessionId.mockImplementation((exists) =>
			exists('last-visited') ? 'last-visited' : null
		);

		await expect(bootstrapHomeSession(deps({ sessions: () => sessions }))).resolves.toBe(true);

		expect(navigateToSession).toHaveBeenCalledWith(sessions[1]);
		expect(startNewChat).not.toHaveBeenCalled();
	});

	it('falls back to the most recently active existing session', async () => {
		const sessions = [session('newer', 3), session('pinned-but-older', 1)];

		await expect(bootstrapHomeSession(deps({ sessions: () => sessions }))).resolves.toBe(true);

		expect(navigateToSession).toHaveBeenCalledWith(sessions[0]);
		expect(startNewChat).not.toHaveBeenCalled();
	});

	it('propagates create failures', async () => {
		startNewChat.mockRejectedValueOnce(new Error('no model'));
		await expect(
			bootstrapHomeSession(deps())
		).rejects.toThrow('no model');
	});
});
