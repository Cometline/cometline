import { describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';
import { applySessionRuntimeEvent, type SessionRuntimeEventDeps } from './session-runtime-events';

function session(id: string): Session {
	return {
		id,
		workspace_id: 'workspace-1',
		workspace_path: '/tmp/workspace',
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
}

function deps(): SessionRuntimeEventDeps {
	return {
		getActiveSessionId: vi.fn().mockReturnValue(null),
		setRunning: vi.fn(),
		refreshTranscript: vi.fn().mockResolvedValue(undefined),
		resumeRun: vi.fn().mockResolvedValue(undefined),
		refreshSession: vi.fn().mockResolvedValue(session('session-1')),
		updateSession: vi.fn()
	};
}

describe('applySessionRuntimeEvent', () => {
	it('updates running state for run lifecycle events', async () => {
		const target = deps();

		await applySessionRuntimeEvent({ type: 'run_started', session_id: 'session-1' }, target);
		await applySessionRuntimeEvent({ type: 'run_finished', session_id: 'session-1' }, target);

		expect(target.setRunning).toHaveBeenNthCalledWith(1, 'session-1', true);
		expect(target.setRunning).toHaveBeenNthCalledWith(2, 'session-1', false);
	});

	it('reloads and follows a run started outside the open session view', async () => {
		const order: string[] = [];
		const target = deps();
		vi.mocked(target.getActiveSessionId).mockReturnValue('session-1');
		vi.mocked(target.refreshTranscript).mockImplementation(async () => {
			order.push('refresh');
		});
		vi.mocked(target.resumeRun).mockImplementation(async () => {
			order.push('resume');
		});

		await applySessionRuntimeEvent({ type: 'run_started', session_id: 'session-1' }, target);

		expect(order).toEqual(['refresh', 'resume']);
	});

	it('does not follow a run for a session that is not open', async () => {
		const target = deps();
		vi.mocked(target.getActiveSessionId).mockReturnValue('session-2');

		await applySessionRuntimeEvent({ type: 'run_started', session_id: 'session-1' }, target);

		expect(target.refreshTranscript).not.toHaveBeenCalled();
		expect(target.resumeRun).not.toHaveBeenCalled();
	});

	it('reloads an open transcript when an external run finishes before attachment', async () => {
		const target = deps();
		vi.mocked(target.getActiveSessionId).mockReturnValue('session-1');

		await applySessionRuntimeEvent({ type: 'run_finished', session_id: 'session-1' }, target);

		expect(target.refreshTranscript).toHaveBeenCalledWith('session-1');
		expect(target.resumeRun).not.toHaveBeenCalled();
	});

	it('reloads transcript and metadata after another process clears a session', async () => {
		const target = deps();

		await applySessionRuntimeEvent(
			{ type: 'session_cleared', session_id: 'session-1' },
			target
		);

		expect(target.refreshTranscript).toHaveBeenCalledWith('session-1');
		expect(target.refreshSession).toHaveBeenCalledWith('session-1');
		expect(target.updateSession).toHaveBeenCalledWith(session('session-1'));
	});
});
