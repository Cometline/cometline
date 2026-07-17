import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { listJobs } from '$lib/client/cometmind';
import { startJobNotificationPoller } from '$lib/jobs/job-notifications';
import { jobsIndicatorStore } from '$lib/stores/jobs-indicator.svelte';

vi.mock('$lib/client/cometmind', () => ({
	listJobs: vi.fn()
}));

describe('startJobNotificationPoller', () => {
	beforeEach(() => {
		jobsIndicatorStore.setOngoingCount(0);
		vi.mocked(listJobs).mockReset();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('updates the sidebar ongoing badge even when notifications are disabled', async () => {
		vi.mocked(listJobs).mockResolvedValue({
			jobs: [
				{
					id: 'j1',
					status: 'ongoing',
					description: 'Ship badge',
					created_at: '',
					updated_at: ''
				},
				{
					id: 'j2',
					status: 'todo',
					description: 'Later',
					created_at: '',
					updated_at: ''
				}
			]
		} as Awaited<ReturnType<typeof listJobs>>);

		const stop = startJobNotificationPoller({
			getSettings: () => ({
				enabled: false,
				onClaimed: true,
				onCompleted: true,
				onReleased: true,
				onBlocked: true
			}),
			onNotify: vi.fn(),
			intervalMs: 60_000
		});

		await vi.waitFor(() => {
			expect(jobsIndicatorStore.ongoingCount).toBe(1);
			expect(jobsIndicatorStore.hasOngoing).toBe(true);
		});
		stop();
	});
});
