import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/client/cometmind', () => ({
	listInboxMessages: vi.fn(async () => ({
		messages: [
			{
				id: 'm1',
				title: 'Hello',
				body: 'World',
				status: 'open',
				process_attempts: 0,
				created_at: 1,
				updated_at: 1
			}
		]
	})),
	getInboxSummary: vi.fn(async () => ({ open_count: 1 })),
	replyInboxMessage: vi.fn(async () => ({
		id: 'm1',
		title: 'Hello',
		body: 'World',
		status: 'archived',
		archive_reason: 'replied',
		process_attempts: 0,
		created_at: 1,
		updated_at: 2
	})),
	dismissInboxMessage: vi.fn(async () => ({
		id: 'm1',
		title: 'Hello',
		body: 'World',
		status: 'archived',
		archive_reason: 'dismissed',
		process_attempts: 0,
		created_at: 1,
		updated_at: 2
	}))
}));

describe('inboxStore', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('loads open messages and summary', async () => {
		const { inboxStore } = await import('./inbox.svelte');
		await inboxStore.load();
		expect(inboxStore.openCount).toBe(1);
		expect(inboxStore.messages).toHaveLength(1);
		expect(inboxStore.messages[0]?.id).toBe('m1');
	});

	it('removes message on reply and dismiss', async () => {
		const { inboxStore } = await import('./inbox.svelte');
		await inboxStore.load();
		await inboxStore.reply('m1', 'ok');
		expect(inboxStore.messages).toHaveLength(0);
		expect(inboxStore.openCount).toBe(0);

		await inboxStore.load();
		await inboxStore.dismiss('m1');
		expect(inboxStore.messages).toHaveLength(0);
	});

	it('applies runtime archive events', async () => {
		const { inboxStore } = await import('./inbox.svelte');
		await inboxStore.load();
		inboxStore.applyArchived('m1', 0);
		expect(inboxStore.messages).toHaveLength(0);
		expect(inboxStore.openCount).toBe(0);
	});
});
