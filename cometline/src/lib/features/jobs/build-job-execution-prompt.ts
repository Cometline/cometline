import type { JobResource } from '$lib/client/cometmind';

export type JobExecutionPromptInput = Pick<JobResource, 'id' | 'description'> &
	Partial<Pick<JobResource, 'definition_of_done' | 'progress'>>;

export function buildJobExecutionPrompt(job: JobExecutionPromptInput): string {
	const dod = job.definition_of_done?.trim() || '(none specified)';
	const progress = job.progress?.trim();
	if (progress) {
		return `Please work on: ${job.description}\n\nDefinition of done: ${dod}\n\nPrevious progress (authoritative resume state):\n${progress}\n\nResume protocol (mandatory order):\n1. If previous progress already satisfies Definition of Done, call complete_job immediately with a final summary. Do NOT re-do finished work and stop on text only.\n2. Otherwise continue ONLY remaining work, call update_job with progress after each meaningful milestone, then complete_job or release_job.\n3. Ending without complete_job or release_job is a protocol violation.\n\n(Use job_id "${job.id}" when calling job tools.)`;
	}
	return `Please work on: ${job.description}\n\nDefinition of done: ${dod}\n\nWhile working, call update_job with progress after each meaningful milestone (and before long tool runs) so another session can resume if this one stops. When finished, call complete_job with a final progress summary. Ending without complete_job or release_job is a protocol violation.\n\n(Use job_id "${job.id}" when calling job tools.)`;
}
