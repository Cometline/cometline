import { truncateWorkspacePath } from '$lib/jobs/group-jobs';
import type { JobResource } from '$lib/client/cometmind';

export function truncateJobLabel(text: string, max = 80): string {
	const trimmed = text.trim();
	if (trimmed.length <= max) return trimmed;
	return `${trimmed.slice(0, max - 1)}…`;
}

export function jobMenuSubtitle(job: {
	priority?: number;
	workspace_path?: string | null;
}): string {
	const parts: string[] = [];
	if ((job.priority ?? 0) > 0) {
		parts.push(`Priority ${job.priority}`);
	}
	const workspace = job.workspace_path?.trim();
	if (workspace) {
		parts.push(truncateWorkspacePath(workspace));
	}
	return parts.join(' · ');
}

export function jobUserDisplayText(job: Pick<JobResource, 'description'>): string {
	const label = truncateJobLabel(job.description, 60);
	return label ? `/job ${label}` : '/job';
}
