import { describe, expect, it } from 'vitest';
import type { Session } from '$lib/types';
import {
	beginAgentModeRequest,
	bindAgentModeForSession,
	completeAgentModeRequest,
	createInitialAgentModeState,
	nextAgentMode
} from './agent-mode-switch';

function session(agent_mode: Session['agent_mode'], id = 'sess-1'): Session {
	return {
		id,
		workspace_id: 'ws',
		workspace_path: '/ws',
		title: 'Chat',
		model_id: 'm',
		provider_id: 'p',
		status: 'active',
		origin: 'user',
		token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
		pinned: false,
		agent_mode,
		created_at: 0,
		updated_at: 1
	};
}

describe('nextAgentMode', () => {
	it('toggles plan and auto', () => {
		expect(nextAgentMode('auto')).toBe('plan');
		expect(nextAgentMode('plan')).toBe('auto');
	});
});

describe('bindAgentModeForSession', () => {
	it('resets when the composer has no session', () => {
		expect(bindAgentModeForSession(session('plan'), '')).toEqual(createInitialAgentModeState());
	});

	it('copies the persisted mode when entering a session', () => {
		expect(bindAgentModeForSession(session('plan'), 'sess-1')).toEqual({
			sessionId: 'sess-1',
			mode: 'plan',
			known: true,
			pending: false,
			queued: null,
			baseline: 'plan'
		});
	});
});

describe('beginAgentModeRequest', () => {
	it('starts a persist when idle and the mode changed', () => {
		const started = beginAgentModeRequest(createInitialAgentModeState('sess-1'), 'plan');
		expect(started.shouldPersist).toBe(true);
		expect(started.state).toEqual({
			sessionId: 'sess-1',
			mode: 'plan',
			known: true,
			pending: true,
			queued: null,
			baseline: 'auto'
		});
	});

	it('queues the latest Tab while a PATCH is in flight', () => {
		const inFlight = beginAgentModeRequest(createInitialAgentModeState('sess-1'), 'plan').state;
		const queued = beginAgentModeRequest(inFlight, 'auto');
		expect(queued.shouldPersist).toBe(false);
		expect(queued.state.mode).toBe('auto');
		expect(queued.state.queued).toBe('auto');
		expect(queued.state.pending).toBe(true);
	});

	it('ignores a no-op when idle', () => {
		const idle = bindAgentModeForSession(session('auto'), 'sess-1');
		const started = beginAgentModeRequest(idle, 'auto');
		expect(started.shouldPersist).toBe(false);
		expect(started.state).toBe(idle);
	});
});

describe('completeAgentModeRequest', () => {
	it('settles on the persisted mode when nothing else was queued', () => {
		const inFlight = beginAgentModeRequest(createInitialAgentModeState('sess-1'), 'plan').state;
		const done = completeAgentModeRequest(inFlight, 'plan', { ok: true, mode: 'plan' });
		expect(done.shouldPersist).toBeNull();
		expect(done.state).toEqual({
			sessionId: 'sess-1',
			mode: 'plan',
			known: true,
			pending: false,
			queued: null,
			baseline: 'plan'
		});
	});

	it('persists the queued opposite mode after the first PATCH returns', () => {
		const inFlight = beginAgentModeRequest(createInitialAgentModeState('sess-1'), 'plan').state;
		const queued = beginAgentModeRequest(inFlight, 'auto').state;
		const done = completeAgentModeRequest(queued, 'plan', { ok: true, mode: 'plan' });
		expect(done.shouldPersist).toBe('auto');
		expect(done.state.pending).toBe(true);
		expect(done.state.mode).toBe('auto');
		expect(done.state.baseline).toBe('plan');
	});

	it('reverts to the last acked mode when the PATCH fails and nothing is queued', () => {
		const inFlight = beginAgentModeRequest(createInitialAgentModeState('sess-1'), 'plan').state;
		const done = completeAgentModeRequest(inFlight, 'plan', { ok: false });
		expect(done.shouldPersist).toBeNull();
		expect(done.state.mode).toBe('auto');
		expect(done.state.pending).toBe(false);
	});

	it('still persists a later Tab when the in-flight PATCH fails', () => {
		const inFlight = beginAgentModeRequest(createInitialAgentModeState('sess-1'), 'plan').state;
		const queued = beginAgentModeRequest(inFlight, 'auto').state;
		const done = completeAgentModeRequest(queued, 'plan', { ok: false });
		expect(done.shouldPersist).toBe('auto');
		expect(done.state.mode).toBe('auto');
		expect(done.state.pending).toBe(true);
	});
});
