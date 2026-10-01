import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
	success: vi.fn(),
	warning: vi.fn(),
	error: vi.fn(),
	goto: vi.fn(),
	getSession: vi.fn(),
	listInboxMessages: vi.fn(),
	openDrawer: vi.fn(),
	drawerOpen: false
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/client/cometmind', () => ({
	getSession: mocks.getSession,
	listInboxMessages: mocks.listInboxMessages,
	listSkillDrafts: vi.fn()
}));
vi.mock('$lib/stores/app-toasts.svelte', () => ({
	appToastStore: {
		success: mocks.success,
		warning: mocks.warning,
		error: mocks.error
	}
}));
vi.mock('$lib/stores/inbox.svelte', () => ({
	inboxStore: {
		get drawerOpen() {
			return mocks.drawerOpen;
		},
		openDrawer: mocks.openDrawer
	}
}));

import { notifyBackgroundRunFinished, notifyConnectionChange, notifyJobActivity, notifyNewInboxMessage, startSkillDraftToastWatch } from './activity-toasts';

const settings = {
	enabled: true,
	onClaimed: false,
	onCompleted: true,
	onReleased: false,
	onBlocked: true
};

describe('activity toasts', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.drawerOpen = false;
	});

	it('toasts completed and blocked jobs, not when notifications are off', () => {
		notifyJobActivity({ kind: 'completed', id: 'job-1', description: 'Ship it' }, settings);
		notifyJobActivity({ kind: 'blocked', id: 'job-2', description: 'Stuck' }, settings);
		notifyJobActivity(
			{ kind: 'completed', id: 'job-3', description: 'Quiet' },
			{ ...settings, enabled: false }
		);

		expect(mocks.success).toHaveBeenCalledWith('Job completed', 'Ship it', expect.any(Function));
		expect(mocks.warning).toHaveBeenCalledWith('Job blocked', 'Stuck', expect.any(Function));
		expect(mocks.success).toHaveBeenCalledTimes(1);
	});

	it('opens the inbox for a new note unless the drawer is already open', async () => {
		mocks.listInboxMessages.mockResolvedValue({
			messages: [{ id: 'm1', title: 'Nightly summary' }]
		});

		await notifyNewInboxMessage('m1');
		mocks.drawerOpen = true;
		await notifyNewInboxMessage('m1');

		expect(mocks.success).toHaveBeenCalledTimes(1);
		expect(mocks.success).toHaveBeenCalledWith(
			'New inbox note',
			'Nightly summary',
			expect.any(Function)
		);
	});

	it('does not toast the first skill draft snapshot, then toasts a new draft', async () => {
		vi.useFakeTimers();
		const listDrafts = vi
			.fn()
			.mockResolvedValueOnce([{ name: 'old', description: 'Already there' }])
			.mockResolvedValueOnce([
				{ name: 'old', description: 'Already there' },
				{ name: 'fresh', description: 'Review me' }
			]);
		const stop = startSkillDraftToastWatch({ intervalMs: 1_000, listDrafts });
		await vi.waitFor(() => expect(listDrafts).toHaveBeenCalledTimes(1));
		expect(mocks.success).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(1_000);
		expect(mocks.success).toHaveBeenCalledWith(
			'Skill draft ready',
			'Review me',
			expect.any(Function)
		);
		stop();
		vi.useRealTimers();
	});

	it('skips background run toasts for the active chat and non-user sessions', async () => {
		mocks.getSession.mockResolvedValue({
			id: 's2',
			title: 'Research',
			origin: 'user'
		});
		await notifyBackgroundRunFinished('s1', 's1');
		await notifyBackgroundRunFinished('s2', 's1');
		mocks.getSession.mockResolvedValueOnce({ id: 's3', title: 'Job', origin: 'autonomy' });
		await notifyBackgroundRunFinished('s3', 's1');

		expect(mocks.success).toHaveBeenCalledTimes(1);
		expect(mocks.success).toHaveBeenCalledWith('Chat finished', 'Research', expect.any(Function));
	});

	it('toasts connection loss and recovery, not startup', () => {
		notifyConnectionChange('connecting', 'ready');
		notifyConnectionChange('ready', 'error');
		notifyConnectionChange('error', 'ready');

		expect(mocks.error).toHaveBeenCalledWith(
			'CometMind disconnected',
			'Reconnecting in the background'
		);
		expect(mocks.success).toHaveBeenCalledWith('CometMind is back');
		expect(mocks.success).toHaveBeenCalledTimes(1);
	});
});
