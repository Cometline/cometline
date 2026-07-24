import { beforeEach, describe, expect, it, vi } from 'vitest';
import { nextSessionAfterDelete } from './next-session-after-delete';
import type { Session } from '$lib/types';

function session(id: string): Session {
	return {
		id,
		workspace_id: 'ws',
		workspace_path: '/ws',
		title: id,
		model_id: 'm',
		provider_id: 'p',
		status: 'active',
		origin: 'user',
		token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
		pinned: false,
		created_at: 0,
		updated_at: 0
	};
}

describe('nextSessionAfterDelete', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('returns null when nothing remains', () => {
		expect(nextSessionAfterDelete('a', [session('a')])).toBeNull();
		expect(nextSessionAfterDelete('a', [])).toBeNull();
	});

	it('prefers the next session in order', () => {
		const ordered = [session('a'), session('b'), session('c')];
		expect(nextSessionAfterDelete('a', ordered)?.id).toBe('b');
		expect(nextSessionAfterDelete('b', ordered)?.id).toBe('c');
	});

	it('falls back to the previous session when deleting the last', () => {
		const ordered = [session('a'), session('b'), session('c')];
		expect(nextSessionAfterDelete('c', ordered)?.id).toBe('b');
	});

	it('picks the first remaining when deleted id is missing', () => {
		expect(nextSessionAfterDelete('missing', [session('a'), session('b')])?.id).toBe('a');
	});
});
