import type { JobResource, ScheduledJobResource } from '#lib/client/cometmind.js';
import { cronDisplayLabel } from '#lib/features/jobs/jobs-page-cron.js';

export function formatClock(ms: number): string {
	if (!ms) return 'Never';
	return new Intl.DateTimeFormat(undefined, {
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit'
	}).format(new Date(ms));
}

export function formatRelativeTime(nowMs: number, ms?: number): string {
	if (!ms) return 'unknown';
	const diff = ms - nowMs;
	const abs = Math.abs(diff);
	if (abs < 5_000) return diff >= 0 ? 'now' : 'just now';
	const units: [number, string][] = [
		[86_400_000, 'd'],
		[3_600_000, 'h'],
		[60_000, 'm'],
		[1_000, 's']
	];
	const [unitMs, label] = units.find(([size]) => abs >= size) ?? units[units.length - 1];
	const value = Math.floor(abs / unitMs);
	return `${value}${label} ${diff >= 0 ? 'left' : 'ago'}`;
}

export function leaseLabel(job: JobResource, nowMs: number): string {
	if (!job.lease_expires_at) return 'No active lease expiry';
	if (job.lease_expires_at <= nowMs) return 'Lease expired';
	return `Lease ${formatRelativeTime(nowMs, job.lease_expires_at)}`;
}

export function progressPreview(job: JobResource): string {
	return job.progress?.trim().split('\n')[0] ?? '';
}

export function sessionLabel(job: JobResource): string {
	return job.assigned_session_id ? job.assigned_session_id.slice(0, 8) : 'unassigned';
}

export function scheduleLabel(job: ScheduledJobResource): string {
	if (job.cron_expr) return cronDisplayLabel(job.cron_expr);
	if (job.run_at) return `one-shot: ${formatClock(job.run_at)}`;
	return 'unscheduled';
}

export function nextRunLabel(job: ScheduledJobResource, nowMs: number): string {
	if (!job.enabled) return 'disabled';
	return formatRelativeTime(nowMs, job.next_run_at);
}
