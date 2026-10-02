import { goto } from '$app/navigation';
import { resolve } from '$app/paths';

/** Opens the Jobs page with jobId selected. */
export function gotoJob(jobId: string): Promise<void> {
	// eslint-disable-next-line svelte/no-navigation-without-resolve -- the pathname is resolved; the rule cannot follow the appended query string
	return goto(`${resolve('jobs')}?job=${encodeURIComponent(jobId)}`);
}
