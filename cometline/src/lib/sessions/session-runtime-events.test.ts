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
		setRunning: vi.fn(),
		refreshTranscript: vi.fn().mockResolvedValue(undefined),
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
