import { describe, expect, it } from 'vitest';
import {
	flattenSessionsInSidebarOrder,
	groupSessionsByWorkspace,
	layoutSessionsForSidebar,
	partitionPinnedSessions,
	sortSessionsByRecency
} from '$lib/sessions/group-by-workspace';
import type { Session } from '$lib/types';

function session(
	id: string,
	workspacePath: string,
	updatedAt: number,
	pinned = false
): Session {
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
		pinned,
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

describe('sortSessionsByRecency', () => {
	it('sorts sessions by updated_at descending', () => {
		const sessions = [
			session('older', '/ws-a', 50),
			session('recent', '/ws-a', 100),
			session('middle', '/ws-a', 80)
		];

		expect(sortSessionsByRecency(sessions).map((item) => item.id)).toEqual([
			'recent',
			'middle',
			'older'
		]);
	});
});

describe('partitionPinnedSessions', () => {
	it('splits pinned and unpinned while preserving order within each list', () => {
		const sessions = [
			session('pinned-a', '/ws-a', 50, true),
			session('unpinned', '/ws-a', 100),
			session('pinned-b', '/ws-a', 40, true)
		];

		expect(partitionPinnedSessions(sessions)).toEqual({
			pinned: [sessions[0], sessions[2]],
			unpinned: [sessions[1]]
		});
	});
});

describe('layoutSessionsForSidebar', () => {
	it('keeps pinned sessions in a global section and removes them from workspace groups', () => {
		const sessions = [
			session('recent', '/ws-a', 100),
			session('pinned-a', '/ws-a', 50, true),
			session('pinned-b', '/ws-b', 60, true),
			session('b1', '/ws-b', 90)
		];

		const layout = layoutSessionsForSidebar(sessions, '/ws-a');

		expect(layout.pinnedSessions.map((item) => item.id)).toEqual(['pinned-b', 'pinned-a']);
		expect(layout.workspaceGroups.map((group) => group.workspacePath)).toEqual([
			'/ws-a',
			'/ws-b'
		]);
		expect(layout.workspaceGroups[0].sessions.map((item) => item.id)).toEqual(['recent']);
		expect(layout.workspaceGroups[1].sessions.map((item) => item.id)).toEqual(['b1']);
	});

	it('partitions Discord gateway sessions into a bottom group when inactive', () => {
		const desktop = session('desktop', '/ws-a', 100);
		const discord = session('discord-1', '/ws-a', 90);
		discord.gateway = { platform: 'discord', channel_id: '123', thread_id: '' };

		const layout = layoutSessionsForSidebar([desktop, discord], '/ws-a', false);

		expect(layout.discordFirst).toBe(false);
		expect(layout.workspaceGroups[0].sessions.map((item) => item.id)).toEqual(['desktop']);
		expect(layout.discordSessions.map((item) => item.id)).toEqual(['discord-1']);
	});

	it('places Discord gateway sessions first when active', () => {
		const desktop = session('desktop', '/ws-a', 100);
		const discord = session('discord-1', '/ws-a', 90);
		discord.gateway = { platform: 'discord', channel_id: '123', thread_id: '' };

		const layout = layoutSessionsForSidebar([desktop, discord], '/ws-a', true);

		expect(layout.discordFirst).toBe(true);
	});

	it('orders Discord before workspace groups in keyboard navigation when active', () => {
		const desktop = session('desktop', '/ws-a', 100);
		const discord = session('discord-1', '/ws-a', 90);
		discord.gateway = { platform: 'discord', channel_id: '123', thread_id: '' };

		const flat = flattenSessionsInSidebarOrder([desktop, discord], '/ws-a', true);
		expect(flat.map((item) => item.id)).toEqual(['discord-1', 'desktop']);
	});

	it('orders Discord after workspace groups in keyboard navigation when inactive', () => {
		const desktop = session('desktop', '/ws-a', 100);
		const discord = session('discord-1', '/ws-a', 90);
		discord.gateway = { platform: 'discord', channel_id: '123', thread_id: '' };

		const flat = flattenSessionsInSidebarOrder([desktop, discord], '/ws-a', false);
		expect(flat.map((item) => item.id)).toEqual(['desktop', 'discord-1']);
	});
});

describe('pinned sidebar order', () => {
	it('puts the global pinned section before workspace groups for keyboard navigation', () => {
		const sessions = [
			session('recent', '/ws-a', 100),
			session('pinned', '/ws-a', 50, true),
			session('b1', '/ws-b', 90)
		];

		const flat = flattenSessionsInSidebarOrder(sessions, '/ws-a');
		expect(flat.map((item) => item.id)).toEqual(['pinned', 'recent', 'b1']);
	});
});
