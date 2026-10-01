import { describe, expect, it } from 'vitest';
import type { JobResource, ScheduledJobResource } from '$lib/client/cometmind';
import {
	formatClock,
	formatRelativeTime,
	leaseLabel,
	nextRunLabel,
	progressPreview,
	scheduleLabel,
	sessionLabel
} from './jobs-page-format';

const NOW = 1_700_000_000_000;

function job(overrides: Partial<JobResource> = {}): JobResource {
	return { id: 'job-1', description: 'Job', status: 'ongoing', ...overrides } as JobResource;
}

function scheduled(overrides: Partial<ScheduledJobResource> = {}): ScheduledJobResource {
	return {
		id: 'sched-1',
		description: 'Job',
		enabled: true,
		...overrides
	} as ScheduledJobResource;
}

describe('formatRelativeTime', () => {
	it('formats past and future offsets', () => {
		expect(formatRelativeTime(NOW)).toBe('unknown');
		expect(formatRelativeTime(NOW, NOW + 2_000)).toBe('now');
		expect(formatRelativeTime(NOW, NOW - 2_000)).toBe('just now');
		expect(formatRelativeTime(NOW, NOW + 90_000)).toBe('1m left');
		expect(formatRelativeTime(NOW, NOW - 2 * 3_600_000)).toBe('2h ago');
	});
});

describe('job labels', () => {
	it('describes lease, progress, and session', () => {
		expect(leaseLabel(job(), NOW)).toBe('No active lease expiry');
		expect(leaseLabel(job({ lease_expires_at: NOW - 1 }), NOW)).toBe('Lease expired');
		expect(leaseLabel(job({ lease_expires_at: NOW + 120_000 }), NOW)).toBe('Lease 2m left');
		expect(progressPreview(job({ progress: '  first\nsecond' }))).toBe('first');
		expect(progressPreview(job())).toBe('');
		expect(sessionLabel(job({ assigned_session_id: 'abcdef123456' }))).toBe('abcdef12');
		expect(sessionLabel(job())).toBe('unassigned');
	});
});

describe('scheduled job labels', () => {
	it('describes schedule and next run', () => {
		expect(scheduleLabel(scheduled({ cron_expr: '0 9 * * *' }))).toBe('Every day at 09:00');
		expect(scheduleLabel(scheduled({ run_at: NOW }))).toBe(`one-shot: ${formatClock(NOW)}`);
		expect(scheduleLabel(scheduled())).toBe('unscheduled');
		expect(nextRunLabel(scheduled({ enabled: false }), NOW)).toBe('disabled');
		expect(nextRunLabel(scheduled({ next_run_at: NOW + 3_600_000 }), NOW)).toBe('1h left');
	});

	it('formats a missing clock time as Never', () => {
		expect(formatClock(0)).toBe('Never');
	});
});
