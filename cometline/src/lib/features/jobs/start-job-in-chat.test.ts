import { describe, expect, it, vi, beforeEach } from 'vitest';
import type { JobResource } from '#lib/client/cometmind.js';

const { claimJob, forkSession, goto, shellStore, sessionStore } = vi.hoisted(() => ({
	claimJob: vi.fn(),
	forkSession: vi.fn(),
	goto: vi.fn(),
	shellStore: { workspacePath: '/session/ws', commitActiveWorkspace: vi.fn() },
	sessionStore: {
		current: { id: 'sess-1' },
		appendSession: vi.fn(),
		queuePendingMessage: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({ goto }));
vi.mock('#lib/client/cometmind.js', () => ({
	buildJobExecutionPrompt: (job: JobResource) => `work on ${job.description}`,
	claimJob: (...args: unknown[]) => claimJob(...args),
	forkSession: (...args: unknown[]) => forkSession(...args),
	createSession: vi.fn()
}));
vi.mock('#lib/stores/shell.svelte.js', () => ({ shellStore }));
vi.mock('#lib/stores/session.svelte.js', () => ({ sessionStore }));
vi.mock('#lib/stores/model.svelte.js', () => ({ modelStore: { selected: null } }));
vi.mock('#lib/features/jobs/format-job-label.js', () => ({
	jobUserDisplayText: (job: JobResource) => `/job ${job.description}`
}));

import { startJobInSession } from './start-job-in-chat';

describe('startJobInSession', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		shellStore.workspacePath = '/session/ws';
		claimJob.mockResolvedValue({
			id: 'job-1',
			description: 'Fix auth',
			workspace_path: '/session/ws'
		});
	});

	it('claims and sends turn when workspace matches', async () => {
		const sendTurn = vi.fn();
		await startJobInSession(
			{ id: 'job-1', description: 'Fix auth', workspace_path: '/session/ws' } as JobResource,
			'sess-1',
			sendTurn
		);
		expect(claimJob).toHaveBeenCalledWith('job-1', 'sess-1');
		expect(sendTurn).toHaveBeenCalled();
		expect(forkSession).not.toHaveBeenCalled();
	});

	it('forks when workspace differs', async () => {
		forkSession.mockResolvedValue({ id: 'sess-2', workspace_path: '/other/ws' });
		claimJob.mockResolvedValue({
			id: 'job-1',
			description: 'Fix auth',
			workspace_path: '/other/ws'
		});
		const sendTurn = vi.fn();
		await startJobInSession(
			{ id: 'job-1', description: 'Fix auth', workspace_path: '/other/ws' } as JobResource,
			'sess-1',
			sendTurn
		);
		expect(forkSession).toHaveBeenCalledWith('sess-1', '/other/ws');
		expect(sessionStore.queuePendingMessage).toHaveBeenCalled();
		expect(goto).toHaveBeenCalledWith('/session/sess-2');
		expect(sendTurn).not.toHaveBeenCalled();
	});
});
