import { goto } from '$app/navigation';
import {
	buildJobExecutionPrompt,
	claimJob,
	createSession,
	type JobResource
} from '$lib/client/cometmind';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { jobUserDisplayText } from '$lib/jobs/format-job-label';

function executionPromptForJob(claimed: JobResource): string {
	let prompt = buildJobExecutionPrompt(claimed);
	const jobPath = claimed.workspace_path?.trim();
	const sessionPath = shellStore.workspacePath?.trim();
	if (jobPath && sessionPath && jobPath !== sessionPath) {
		prompt +=
			`\n\nNote: this job targets workspace \`${jobPath}\` but this session uses \`${sessionPath}\`. Consider /change to fork into the correct workspace before editing files.`;
	}
	return prompt;
}

export async function startJobInChat(job: JobResource): Promise<void> {
	const current = sessionStore.current;
	if (current) {
		const claimed = await claimJob(job.id, current.id);
		const prompt = executionPromptForJob(claimed);
		sessionStore.queuePendingMessage(current.id, prompt, undefined, undefined, jobUserDisplayText(claimed));
		await goto(`/session/${current.id}`);
		return;
	}

	const selectedModel = modelStore.selected;
	if (!selectedModel) {
		throw new Error('Select a model before starting a job.');
	}

	const session = await createSession({
		workspace_path: shellStore.workspacePath,
		model_id: selectedModel.modelId,
		provider_id: selectedModel.providerId
	});
	const claimed = await claimJob(job.id, session.id);
	const prompt = executionPromptForJob(claimed);
	sessionStore.appendSession(session);
	sessionStore.queuePendingMessage(session.id, prompt, undefined, undefined, jobUserDisplayText(claimed));
	shellStore.migrateDraftPanel(session.id);
	await goto(`/session/${session.id}`);
}
