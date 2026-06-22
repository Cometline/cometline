import { goto } from '$app/navigation';
import {
	buildJobExecutionPrompt,
	claimJob,
	createSession,
	forkSession,
	type JobResource
} from '$lib/client/cometmind';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { jobUserDisplayText } from '$lib/jobs/format-job-label';

export type JobStartSender = (payload: ChatTurnPayload) => void | Promise<void>;

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

async function sendViaQueue(sessionId: string, payload: ChatTurnPayload): Promise<void> {
	sessionStore.queuePendingMessage(
		sessionId,
		payload.text,
		payload.images,
		payload.filePaths,
		payload.displayText
	);
	await goto(`/session/${sessionId}`);
}

/** Claim and start a job in-session, forking when the job workspace differs. */
export async function startJobInSession(
	job: JobResource,
	sessionId: string,
	sendTurn: JobStartSender
): Promise<void> {
	const jobPath = job.workspace_path?.trim() ?? '';
	const sessionPath = shellStore.workspacePath?.trim() ?? '';

	if (jobPath && sessionPath && jobPath !== sessionPath) {
		const forked = await forkSession(sessionId, jobPath);
		sessionStore.appendSession(forked);
		shellStore.commitActiveWorkspace(jobPath);
		const claimed = await claimJob(job.id, forked.id);
		const prompt = executionPromptForJob(claimed);
		sessionStore.queuePendingMessage(
			forked.id,
			prompt,
			undefined,
			undefined,
			jobUserDisplayText(claimed)
		);
		await goto(`/session/${forked.id}`);
		return;
	}

	const claimed = await claimJob(job.id, sessionId);
	const prompt = executionPromptForJob(claimed);
	await sendTurn({ text: prompt, displayText: jobUserDisplayText(claimed) });
}

export async function startJobInChat(job: JobResource): Promise<void> {
	const current = sessionStore.current;
	if (current) {
		await startJobInSession(job, current.id, (payload) => sendViaQueue(current.id, payload));
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
	sessionStore.appendSession(session);
	shellStore.migrateDraftPanel(session.id);
	await startJobInSession(job, session.id, (payload) => sendViaQueue(session.id, payload));
}
