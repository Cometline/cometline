import { beforeEach, describe, expect, it, vi } from 'vitest';

const getJob = vi.fn();
const getSession = vi.fn();

vi.mock('$lib/client/cometmind', () => ({
	getJob: (...args: unknown[]) => getJob(...args),
	getSession: (...args: unknown[]) => getSession(...args)
}));

describe('inbox link availability', () => {
	beforeEach(() => {
		vi.resetModules();
		getJob.mockReset();
		getSession.mockReset();
	});

	it('collects unique job and session keys', async () => {
		const { collectInboxLinkKeys } = await import('./link-availability');
		expect(
			collectInboxLinkKeys([
				{ job_id: 'j1', session_id: 's1' },
				{ job_id: 'j1', session_id: 's2' },
				{ job_id: '  ', session_id: undefined }
			]).sort()
		).toEqual(['job:j1', 'session:s1', 'session:s2']);
	});

	it('marks missing targets after probe failures', async () => {
		getJob.mockResolvedValueOnce({ id: 'j1' });
		getSession.mockRejectedValueOnce(new Error('not found'));
		const { resolveInboxLinkAvailability } = await import('./link-availability');
		const map = await resolveInboxLinkAvailability([
			{ job_id: 'j1', session_id: 's-gone' }
		]);
		expect(map['job:j1']).toBe('available');
		expect(map['session:s-gone']).toBe('missing');
	});
});
