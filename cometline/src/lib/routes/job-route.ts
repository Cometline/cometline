import { goto } from '$app/navigation';
import { resolve } from '$app/paths';

/** Opens the Jobs page with jobId selected. */
export function gotoJob(jobId: string): Promise<void> {
	return goto(`${resolve('/jobs')}?job=${encodeURIComponent(jobId)}`);
}
