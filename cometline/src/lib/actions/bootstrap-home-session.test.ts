import { beforeEach, describe, expect, it, vi } from 'vitest';
import { bootstrapHomeSession } from './bootstrap-home-session';

describe('bootstrapHomeSession', () => {
	const startNewChat = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		startNewChat.mockResolvedValue(undefined);
	});

	it('skips when the sidecar is not ready', async () => {
		await expect(
			bootstrapHomeSession({
				connectionStatus: () => 'connecting',
				workspacePath: () => '/ws',
				startNewChat
			})
		).resolves.toBe(false);
		expect(startNewChat).not.toHaveBeenCalled();
	});

	it('skips when workspace is unset', async () => {
		await expect(
			bootstrapHomeSession({
				connectionStatus: () => 'ready',
				workspacePath: () => '/',
				startNewChat
			})
		).resolves.toBe(false);
		expect(startNewChat).not.toHaveBeenCalled();
	});

	it('creates a session when ready', async () => {
		await expect(
			bootstrapHomeSession({
				connectionStatus: () => 'ready',
				workspacePath: () => '/ws',
				startNewChat
			})
		).resolves.toBe(true);
		expect(startNewChat).toHaveBeenCalledOnce();
	});

	it('propagates create failures', async () => {
		startNewChat.mockRejectedValueOnce(new Error('no model'));
		await expect(
			bootstrapHomeSession({
				connectionStatus: () => 'ready',
				workspacePath: () => '/ws',
				startNewChat
			})
		).rejects.toThrow('no model');
	});
});
