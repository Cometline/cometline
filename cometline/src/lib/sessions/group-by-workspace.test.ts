import { describe, expect, it } from 'vitest';
import {
	flattenSessionsInSidebarOrder,
	groupSessionsByWorkspace
} from '$lib/sessions/group-by-workspace';
import type { Session } from '$lib/types';

function session(id: string, workspacePath: string, updatedAt: number): Session {
	return {
		id,
		workspace_id: `ws-${workspacePath}`,
		workspace_path: workspacePath,
		title: id,
		model_id: 'm',
		provider_id: 'p',
		status: 'active',
		token_usage: {
			input_tokens: 0,
			output_tokens: 0,
			cache_read: 0,
			cache_write: 0
		},
		created_at: updatedAt,
		updated_at: updatedAt
	};
}

describe('flattenSessionsInSidebarOrder', () => {
	it('walks committed workspace sessions before other workspaces', () => {
		const sessions = [
			session('b1', '/ws-b', 90),
			session('a1', '/ws-a', 100),
			session('a2', '/ws-a', 80),
			session('c1', '/ws-c', 70)
		];

		const flat = flattenSessionsInSidebarOrder(sessions, '/ws-a');

		expect(flat.map((item) => item.id)).toEqual(['a1', 'a2', 'b1', 'c1']);
		expect(groupSessionsByWorkspace(sessions, '/ws-a').map((g) => g.workspacePath)).toEqual([
			'/ws-a',
			'/ws-b',
			'/ws-c'
		]);
	});
});
