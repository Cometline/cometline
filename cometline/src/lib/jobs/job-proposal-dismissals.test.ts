import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
	dismissJobProposal,
	getJobProposalDismissal,
	isJobProposalDismissed,
	jobProposalDismissalSummary,
	jobProposalFingerprint
} from '$lib/jobs/job-proposal-dismissals';
import type { JobProposal } from '$lib/jobs/parse-job-proposal';

const proposal: JobProposal = {
	description: 'Say hello',
	definitionOfDone: 'Job has been created',
	defaultWorkspace: '/tmp/ws'
};

function installLocalStorageMock() {
	const store = new Map<string, string>();
	const mock = {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => {
			store.set(key, value);
		},
		removeItem: (key: string) => {
			store.delete(key);
		},
		clear: () => {
			store.clear();
		}
	};
	Object.defineProperty(globalThis, 'localStorage', { value: mock, configurable: true });
	return store;
}

describe('job-proposal-dismissals', () => {
	beforeEach(() => {
		installLocalStorageMock();
	});

	afterEach(() => {
		localStorage.clear();
	});

	it('fingerprints by description and definition of done', () => {
		expect(jobProposalFingerprint(proposal)).toBe('Say hello\0Job has been created');
	});

	it('persists and reads dismissal by session', () => {
		expect(isJobProposalDismissed('sess-1', proposal)).toBe(false);
		dismissJobProposal('sess-1', proposal, { action: 'cancelled' });
		expect(isJobProposalDismissed('sess-1', proposal)).toBe(true);
		expect(isJobProposalDismissed('sess-2', proposal)).toBe(false);
	});

	it('stores created job id', () => {
		dismissJobProposal('sess-1', proposal, { action: 'created', jobId: 'job-42' });
		const record = getJobProposalDismissal('sess-1', proposal);
		expect(record?.action).toBe('created');
		expect(record?.jobId).toBe('job-42');
		expect(jobProposalDismissalSummary(record!)).toBe('Job created (job-42).');
	});

	it('summarizes cancelled dismissal', () => {
		const record = dismissJobProposal('sess-1', proposal, { action: 'cancelled' });
		expect(jobProposalDismissalSummary(record)).toBe('Job proposal dismissed.');
	});
});
