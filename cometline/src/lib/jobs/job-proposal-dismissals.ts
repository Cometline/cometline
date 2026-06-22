import type { JobProposal } from '$lib/jobs/parse-job-proposal';

const STORAGE_KEY = 'cometline.job-proposal-dismissals';

export type JobProposalDismissAction = 'cancelled' | 'created';

export type JobProposalDismissal = {
	action: JobProposalDismissAction;
	jobId?: string;
	at: number;
};

type SessionDismissals = Record<string, JobProposalDismissal>;
type AllDismissals = Record<string, SessionDismissals>;

function storageAvailable(): boolean {
	return typeof localStorage !== 'undefined';
}

function readAll(): AllDismissals {
	if (!storageAvailable()) return {};
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return {};
		const parsed: unknown = JSON.parse(raw);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return parsed as AllDismissals;
		}
	} catch {
		return {};
	}
	return {};
}

function writeAll(all: AllDismissals) {
	if (!storageAvailable()) return;
	localStorage.setItem(STORAGE_KEY, JSON.stringify(all));
}

/** Stable key for a proposal within a session (survives transcript item id changes). */
export function jobProposalFingerprint(proposal: JobProposal): string {
	return `${proposal.description}\0${proposal.definitionOfDone}`;
}

export function getJobProposalDismissal(
	sessionId: string,
	proposal: JobProposal
): JobProposalDismissal | null {
	if (!sessionId.trim()) return null;
	const fingerprint = jobProposalFingerprint(proposal);
	return readAll()[sessionId]?.[fingerprint] ?? null;
}

export function isJobProposalDismissed(sessionId: string, proposal: JobProposal): boolean {
	return getJobProposalDismissal(sessionId, proposal) != null;
}

export function dismissJobProposal(
	sessionId: string,
	proposal: JobProposal,
	dismissal: Pick<JobProposalDismissal, 'action' | 'jobId'>
): JobProposalDismissal {
	const fingerprint = jobProposalFingerprint(proposal);
	const record: JobProposalDismissal = {
		action: dismissal.action,
		jobId: dismissal.jobId,
		at: Date.now()
	};
	const all = readAll();
	const session = { ...(all[sessionId] ?? {}), [fingerprint]: record };
	writeAll({ ...all, [sessionId]: session });
	return record;
}

export function jobProposalDismissalSummary(dismissal: JobProposalDismissal): string {
	if (dismissal.action === 'created') {
		return dismissal.jobId ? `Job created (${dismissal.jobId}).` : 'Job created.';
	}
	return 'Job proposal dismissed.';
}
