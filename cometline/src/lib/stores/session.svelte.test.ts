import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

vi.mock('$app/environment', () => ({ browser: false }));

function session(overrides: Partial<Session> = {}): Session {
	return {
		id: 'sess-1',
		workspace_id: 'ws',
		workspace_path: '/ws',
		title: 'Chat',
		model_id: 'm',
		provider_id: 'p',
		status: 'active',
		origin: 'user',
		token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
		pinned: false,
		agent_mode: 'auto',
		created_at: 0,
		updated_at: 100,
		...overrides
	};
}

describe('sessionStore metadata patches', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	afterEach(() => {
		vi.resetModules();
	});

	it('refreshes title without rolling back agent mode', async () => {
		const { sessionStore } = await import('./session.svelte');
		sessionStore.setSessions([session({ agent_mode: 'plan', title: '', updated_at: 300 })]);
		sessionStore.patchSessionMetadata('sess-1', {
			title: 'Named',
			token_usage: { input_tokens: 4, output_tokens: 0, cache_read: 0, cache_write: 0 },
			updated_at: 100
		});

		expect(sessionStore.sessions[0]).toMatchObject({
			title: 'Named',
			agent_mode: 'plan',
			token_usage: { input_tokens: 4, output_tokens: 0, cache_read: 0, cache_write: 0 },
			updated_at: 100
		});
	});

	it('lets an explicit session write replace agent mode', async () => {
		const { sessionStore } = await import('./session.svelte');
		sessionStore.setSessions([session({ agent_mode: 'plan' })]);
		sessionStore.updateSession(
			session({ agent_mode: 'auto', title: 'Named', updated_at: 200 })
		);

		expect(sessionStore.sessions[0]).toMatchObject({
			title: 'Named',
			agent_mode: 'auto',
			updated_at: 200
		});
	});
});
