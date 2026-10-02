import { listJobs } from '#lib/client/cometmind.js';
import type { CometMindJobsNotificationSettings } from '#lib/cometmind-settings.js';
import { jobsIndicatorStore } from '#lib/stores/jobs-indicator.svelte.js';

type JobSnapshot = {
	id: string;
	status: string;
	assigned_session_id?: string | null;
	failure_count?: number;
	description: string;
};

export function startJobNotificationPoller(opts: {
	getSettings: () => CometMindJobsNotificationSettings;
	intervalMs?: number;
	onNotify: (title: string, body: string, job?: JobSnapshot) => void;
}): () => void {
	const intervalMs = opts.intervalMs ?? 30_000;
	const last = new Map<string, JobSnapshot>();

	async function poll() {
		const settings = opts.getSettings();
		try {
			const res = await listJobs();
			let ongoing = 0;
			for (const job of res.jobs ?? []) {
				if (job.deleted_at || job.archived_at) continue;
				if (job.status === 'ongoing') ongoing += 1;
				const snap: JobSnapshot = {
					id: job.id,
					status: job.status,
					assigned_session_id: job.assigned_session_id,
					failure_count: job.failure_count,
					description: job.description
				};
				const prev = last.get(job.id);
				if (settings.enabled && prev) {
					if (
						settings.onClaimed &&
						!prev.assigned_session_id &&
						job.assigned_session_id &&
						job.status === 'ongoing'
					) {
						opts.onNotify('Job claimed', job.description, snap);
					}
					if (settings.onCompleted && prev.status !== 'done' && job.status === 'done') {
						opts.onNotify('Job completed', job.description, snap);
					}
					if (settings.onReleased && prev.status === 'ongoing' && job.status === 'todo') {
						opts.onNotify('Job released', job.description, snap);
					}
					if (
						settings.onBlocked &&
						prev.status !== 'blocked' &&
						job.status === 'blocked'
					) {
						opts.onNotify('Job blocked', job.description, snap);
					}
				}
				last.set(job.id, snap);
			}
			jobsIndicatorStore.setOngoingCount(ongoing);
		} catch {
			// Sidecar may be offline.
		}
	}

	void poll();
	const timer = setInterval(() => void poll(), intervalMs);
	return () => clearInterval(timer);
}
