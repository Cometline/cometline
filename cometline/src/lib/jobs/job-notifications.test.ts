import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { listJobs, type JobResource } from '$lib/client/cometmind';
import { startJobNotificationPoller } from '$lib/jobs/job-notifications';
import { jobsIndicatorStore } from '$lib/stores/jobs-indicator.svelte';

vi.mock('$lib/client/cometmind', () => ({
	listJobs: vi.fn()
}));

function job(overrides: Partial<JobResource> & Pick<JobResource, 'id' | 'status'>): JobResource {
	return {
		description: 'test',
		definition_of_done: '',
		progress: '',
		created_by: 'user',
		failure_count: 0,
		created_at: 1,
		updated_at: 1,
		...overrides
	};
}

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
				job({ id: 'j1', status: 'ongoing', description: 'Ship badge' }),
				job({ id: 'j2', status: 'todo', description: 'Later' })
			]
		});

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
