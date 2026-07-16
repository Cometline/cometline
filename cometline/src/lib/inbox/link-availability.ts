import { getJob, getSession } from '$lib/client/cometmind';

export type LinkKind = 'job' | 'session';

export type LinkAvailability = 'unknown' | 'available' | 'missing';

export type LinkAvailabilityMap = Record<string, LinkAvailability>;

function linkKey(kind: LinkKind, id: string): string {
	return `${kind}:${id}`;
}

export function collectInboxLinkKeys(
	messages: Array<{ job_id?: string; session_id?: string }>
): string[] {
	const keys = new Set<string>();
	for (const message of messages) {
		const jobId = message.job_id?.trim();
		if (jobId) keys.add(linkKey('job', jobId));
		const sessionId = message.session_id?.trim();
		if (sessionId) keys.add(linkKey('session', sessionId));
	}
	return [...keys];
}

async function probeLink(kind: LinkKind, id: string): Promise<LinkAvailability> {
	try {
		if (kind === 'job') {
			await getJob(id);
		} else {
			await getSession(id);
		}
		return 'available';
	} catch {
		return 'missing';
	}
}

/** Probe job/session deep links; returns a map keyed as `job:<id>` / `session:<id>`. */
export async function resolveInboxLinkAvailability(
	messages: Array<{ job_id?: string; session_id?: string }>,
	options: { signal?: AbortSignal } = {}
): Promise<LinkAvailabilityMap> {
	const keys = collectInboxLinkKeys(messages);
	const out: LinkAvailabilityMap = {};
	await Promise.all(
		keys.map(async (key) => {
			if (options.signal?.aborted) return;
			const [kind, ...rest] = key.split(':');
			const id = rest.join(':');
			if ((kind !== 'job' && kind !== 'session') || !id) return;
			const status = await probeLink(kind, id);
			if (options.signal?.aborted) return;
			out[key] = status;
		})
	);
	if (options.signal?.aborted) return {};
	return out;
}

export function jobLinkKey(jobId: string): string {
	return linkKey('job', jobId);
}

export function sessionLinkKey(sessionId: string): string {
	return linkKey('session', sessionId);
}
