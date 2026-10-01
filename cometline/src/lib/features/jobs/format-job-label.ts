import { truncateWorkspacePath } from '$lib/features/jobs/group-jobs';
import type { JobResource } from '$lib/client/cometmind';

export function truncateJobLabel(text: string, max = 80): string {
	const trimmed = text.trim();
	if (trimmed.length <= max) return trimmed;
	return `${trimmed.slice(0, max - 1)}…`;
}

export function jobMenuSubtitle(job: { workspace_path?: string | null }): string {
	const workspace = job.workspace_path?.trim();
	if (!workspace) return '';
	return truncateWorkspacePath(workspace);
}

export function jobUserDisplayText(job: Pick<JobResource, 'description'>): string {
	const label = truncateJobLabel(job.description, 60);
	return label ? `/job ${label}` : '/job';
}
