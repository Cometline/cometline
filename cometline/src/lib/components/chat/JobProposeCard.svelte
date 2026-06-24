<script lang="ts">
	import { LoaderCircle } from '@lucide/svelte';
	import WorkspacePathField from '$lib/components/WorkspacePathField.svelte';
	import { createJob, type JobResource } from '$lib/client/cometmind';
	import type { JobProposal } from '$lib/jobs/parse-job-proposal';
	import { shellStore } from '$lib/stores/shell.svelte';
	import type { ChatTurnPayload } from '$lib/actions/start-chat';

	import type { JobProposalDismissAction } from '$lib/jobs/job-proposal-dismissals';

	type CardPhase = 'idle' | 'creating' | 'created' | 'starting' | 'cancelled' | 'error';

	let {
		proposal,
		sessionId: _sessionId,
		onNotifyAgent,
		onStartJob,
		onDismiss
	}: {
		proposal: JobProposal;
		sessionId: string;
		onNotifyAgent?: (payload: ChatTurnPayload) => void | Promise<void>;
		onStartJob?: (job: JobResource) => void | Promise<void>;
		onDismiss?: (action: JobProposalDismissAction, jobId?: string) => void;
	} = $props();

	let workspacePath = $state('');
	const defaultWorkspacePath = $derived(
		proposal.defaultWorkspace.trim() || shellStore.workspacePath?.trim() || ''
	);
	const selectedWorkspacePath = $derived(workspacePath || defaultWorkspacePath);

	$effect.pre(() => {
		if (!workspacePath && defaultWorkspacePath) {
			workspacePath = defaultWorkspacePath;
		}
	});
	let phase = $state<CardPhase>('idle');
	let error = $state('');
	let createdJob = $state<JobResource | null>(null);

	async function handleCreate() {
		if (phase !== 'idle') return;
		phase = 'creating';
		error = '';
		try {
			const job = await createJob({
				description: proposal.description,
				definition_of_done: proposal.definitionOfDone || undefined,
				workspace_path: selectedWorkspacePath.trim() || undefined,
				created_by: 'user',
				source_platform: 'desktop'
			});
			createdJob = job;
			phase = 'created';
			onDismiss?.('created', job.id);
		} catch (err) {
			phase = 'error';
			error = err instanceof Error ? err.message : 'Failed to create job';
		}
	}

	function handleCancel() {
		if (phase === 'creating' || phase === 'starting') return;
		phase = 'cancelled';
		onDismiss?.('cancelled');
	}

	async function handleStartNow() {
		if (!createdJob || phase !== 'created') return;
		if (!onStartJob) {
			error = 'Cannot start job from here.';
			return;
		}
		phase = 'starting';
		error = '';
		try {
			await onStartJob(createdJob);
		} catch (err) {
			phase = 'created';
			error = err instanceof Error ? err.message : 'Failed to start job';
		}
	}

	async function handleDone() {
		if (!createdJob || phase !== 'created') return;
		const path =
			createdJob.workspace_path?.trim() || selectedWorkspacePath.trim() || 'unspecified';
		await onNotifyAgent?.({
			text: `Created job ${createdJob.id} in workspace ${path}.`,
			displayText: `Created job: ${proposal.description}`
		});
		phase = 'cancelled';
		onDismiss?.('created', createdJob.id);
	}
</script>

<div class="job-propose-card settings-ui">
	<p class="card-eyebrow">Confirm job</p>
	<div class="proposal-fields">
		<div class="proposal-field">
			<span class="field-label">Description</span>
			<p class="field-value">{proposal.description}</p>
		</div>
		{#if proposal.definitionOfDone}
			<div class="proposal-field">
				<span class="field-label">Definition of done</span>
				<p class="field-value">{proposal.definitionOfDone}</p>
			</div>
		{/if}
		<div class="proposal-field">
			<span class="field-label">Workspace</span>
			<WorkspacePathField bind:value={workspacePath} disabled={phase !== 'idle'} />
		</div>
	</div>

	{#if error}
		<p class="card-error">{error}</p>
	{/if}

	{#if phase === 'idle'}
		<div class="card-actions">
			<button type="button" class="secondary" onclick={handleCancel}>Cancel</button>
			<button type="button" class="primary" onclick={() => void handleCreate()}
				>Create job</button
			>
		</div>
	{:else if phase === 'creating' || phase === 'starting'}
		<div class="card-status">
			<LoaderCircle size={14} class="spin" />
			<span>{phase === 'creating' ? 'Creating job…' : 'Starting job…'}</span>
		</div>
	{:else if phase === 'created' && createdJob}
		<div class="created-meta">
			<p>Created <code>{createdJob.id}</code></p>
		</div>
		<div class="card-actions">
			<button type="button" class="secondary" onclick={() => void handleDone()}>Done</button>
			<button type="button" class="primary" onclick={() => void handleStartNow()}
				>Start now</button
			>
		</div>
	{:else if phase === 'cancelled'}
		<p class="card-muted">Dismissed.</p>
	{:else if phase === 'error'}
		<div class="card-actions">
			<button type="button" class="secondary" onclick={() => (phase = 'idle')}
				>Try again</button
			>
		</div>
	{/if}
</div>

<style>
	.job-propose-card {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.card-eyebrow {
		margin: 0;
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-muted);
	}

	.proposal-fields {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.proposal-field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.field-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	.field-value {
		margin: 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-main);
		white-space: pre-wrap;
	}

	.card-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.card-status {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.created-meta {
		font-size: 12px;
		color: var(--text-muted);
	}

	.created-meta p {
		margin: 0;
	}

	.created-meta code {
		font-size: 11px;
	}

	.card-error {
		margin: 0;
		font-size: 12px;
		color: #b42318;
	}

	.card-muted {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	:global(.job-propose-card .spin) {
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
