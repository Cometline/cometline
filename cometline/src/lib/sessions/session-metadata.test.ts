import { describe, expect, it } from 'vitest';
import type { Session } from '$lib/types';
import { applySessionMetadata, normalizeAgentMode } from './session-metadata';

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

describe('normalizeAgentMode', () => {
	it('treats only plan as plan', () => {
		expect(normalizeAgentMode('plan')).toBe('plan');
		expect(normalizeAgentMode('auto')).toBe('auto');
		expect(normalizeAgentMode(undefined)).toBe('auto');
	});
});

describe('applySessionMetadata', () => {
	it('updates title and usage without touching agent mode', () => {
		const existing = session({ agent_mode: 'plan', title: '', updated_at: 300 });
		expect(
			applySessionMetadata(existing, {
				title: 'Named',
				token_usage: { input_tokens: 1, output_tokens: 2, cache_read: 0, cache_write: 0 },
				updated_at: 100
			})
		).toEqual({
			...existing,
			title: 'Named',
			token_usage: { input_tokens: 1, output_tokens: 2, cache_read: 0, cache_write: 0 },
			updated_at: 100
		});
	});
});
