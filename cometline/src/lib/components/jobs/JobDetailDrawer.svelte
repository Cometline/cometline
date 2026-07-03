<script lang="ts">
	import { fly, fade } from 'svelte/transition';
	import { onMount } from 'svelte';
	import { Archive, RotateCcw, X, Trash2, Play, ExternalLink, RefreshCw } from '@lucide/svelte';
	import {
		getSession,
		type JobEventResource,
		type JobResource,
		type SessionPlanResponse,
		type SessionPlanStep
	} from '$lib/client/cometmind';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import JobCreateForm from './JobCreateForm.svelte';
	import WorkspacePathField from '$lib/components/WorkspacePathField.svelte';
	import { startJobInChat } from '$lib/jobs/start-job-in-chat';

	type DrawerMode = 'detail' | 'create';

	let {
		job = null,
		mode = 'detail',
		events = [],
		saving = false,
		loadingEvents = false,
		sessionPlan = null,
		loadingSessionPlan = false,
		editDescription = $bindable(''),
		editDod = $bindable(''),
		editWorkspacePath = $bindable(''),
		createDescription = $bindable(''),
		createDod = $bindable(''),
		createWorkspacePath = $bindable(''),
		onClose,
		onSave,
		onDelete,
		onArchive,
		onUnarchive,
		onRetry,
		onCreate,
		onStartInChat
	}: {
		job?: JobResource | null;
		mode?: DrawerMode;
		events?: JobEventResource[];
		saving?: boolean;
		loadingEvents?: boolean;
		sessionPlan?: SessionPlanResponse | null;
		loadingSessionPlan?: boolean;
		editDescription?: string;
		editDod?: string;
		editWorkspacePath?: string;
		createDescription?: string;
		createDod?: string;
		createWorkspacePath?: string;
		onClose: () => void;
		onSave?: () => void | Promise<void>;
		onDelete?: (job: JobResource) => void | Promise<void>;
		onArchive?: (job: JobResource) => void | Promise<void>;
		onUnarchive?: (job: JobResource) => void | Promise<void>;
		onRetry?: (job: JobResource) => void | Promise<void>;
		onCreate?: () => void | Promise<void>;
		onStartInChat?: (job: JobResource) => void | Promise<void>;
	} = $props();

	let starting = $state(false);
	let startError = $state('');
	let openingSession = $state(false);
	let openSessionError = $state('');
	const isArchived = $derived(job?.archived_at != null);
	const isBlocked = $derived(job?.status === 'blocked');

	onMount(() => {
		function onKeydown(event: KeyboardEvent) {
			if (event.key === 'Escape') {
				event.preventDefault();
				onClose();
			}
		}
		window.addEventListener('keydown', onKeydown);
		return () => window.removeEventListener('keydown', onKeydown);
	});

	async function handleStartInChat() {
		if (!job || job.status !== 'todo') return;
		starting = true;
		startError = '';
		try {
			if (onStartInChat) {
				await onStartInChat(job);
			} else {
				await startJobInChat(job);
			}
		} catch (err) {
			startError = err instanceof Error ? err.message : 'Failed to start job';
		} finally {
			starting = false;
		}
	}

	async function handleOpenRunSession() {
		if (!job?.assigned_session_id) return;
		openingSession = true;
		openSessionError = '';
		try {
			const session = await getSession(job.assigned_session_id);
			navigateToSession(session);
			onClose();
		} catch (err) {
			openSessionError = err instanceof Error ? err.message : 'Failed to open run session';
		} finally {
			openingSession = false;
		}
	}

	function formatRetryTime(ms?: number): string {
		if (!ms) return 'not scheduled';
		return new Intl.DateTimeFormat(undefined, {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		}).format(new Date(ms));
	}

	function statusLabel(status: SessionPlanStep['status']) {
		return status.replace('_', ' ');
	}
</script>

<button
	class="drawer-scrim"
	aria-label="Close job details"
	onclick={onClose}
	transition:fade={{ duration: 120 }}
></button>

<aside class="job-drawer settings-ui" transition:fly={{ x: 320, duration: 200 }}>
	<header class="drawer-header">
		<div>
			<p class="drawer-eyebrow">{mode === 'create' ? 'New job' : (job?.status ?? '')}</p>
			<h2>{mode === 'create' ? 'Create job' : (job?.description ?? 'Job')}</h2>
		</div>
		<button type="button" class="secondary icon-only" aria-label="Close" onclick={onClose}>
			<X size={16} />
		</button>
	</header>

	<div class="drawer-body scrollbar-none">
		{#if mode === 'create'}
			<JobCreateForm
				bind:description={createDescription}
				bind:dod={createDod}
				bind:workspacePath={createWorkspacePath}
				{saving}
				onSubmit={() => onCreate?.()}
			/>
		{:else if job}
			<div class="drawer-meta">
				<p><span>Status</span> {job.status}</p>
				{#if job.failure_count > 0}
					<p><span>Failures</span> {job.failure_count}</p>
				{/if}
				{#if job.next_retry_at}
					<p><span>Next retry</span> {formatRetryTime(job.next_retry_at)}</p>
				{/if}
				{#if job.assigned_session_id}
					<p><span>Assigned</span> <code>{job.assigned_session_id}</code></p>
				{/if}
				{#if job.workspace_path}
					<p><span>Workspace</span> <code>{job.workspace_path}</code></p>
				{/if}
			</div>

			{#if isBlocked}
				<section class="drawer-section blocked-section">
					<h3>Blocked</h3>
					<p class="drawer-copy">
						This job reached the retry limit. Review the latest failure, then retry it
						when the underlying issue is fixed.
					</p>
					{#if job.last_failure_reason}
						<pre class="drawer-pre">{job.last_failure_reason}</pre>
					{/if}
				</section>
			{/if}

			{#if job.progress?.trim()}
				<section class="drawer-section">
					<h3>Progress</h3>
					<pre class="drawer-pre">{job.progress}</pre>
				</section>
			{/if}

			{#if job.status === 'ongoing'}
				<section class="drawer-section">
					<h3>Current plan</h3>
					{#if loadingSessionPlan}
						<p class="drawer-muted">Loading checklist…</p>
					{:else if sessionPlan?.dismissed}
						<p class="drawer-muted">Checklist hidden for this session.</p>
					{:else if sessionPlan?.steps?.length}
						<ol class="plan-steps">
							{#each sessionPlan.steps as step (step.id)}
								<li class="plan-step" class:completed={step.status === 'completed'}>
									<div class="plan-step-line">
										<span class="plan-step-description">{step.description}</span>
										<span class="plan-step-status" data-status={step.status}
											>{statusLabel(step.status)}</span
										>
									</div>
									{#if step.status === 'blocked' && step.blocker_reason}
										<p class="drawer-muted">{step.blocker_reason}</p>
									{/if}
								</li>
							{/each}
						</ol>
					{:else}
						<p class="drawer-muted">No structured checklist yet.</p>
					{/if}
				</section>
			{/if}

			{#if job.definition_of_done?.trim() && job.status !== 'todo'}
				<section class="drawer-section">
					<h3>Definition of done</h3>
					<p class="drawer-copy">{job.definition_of_done}</p>
				</section>
			{/if}

			{#if job.status === 'todo' && !isArchived}
				<section class="drawer-section">
					<h3>Edit</h3>
					<form
						class="drawer-form"
						onsubmit={(e) => {
							e.preventDefault();
							void onSave?.();
						}}
					>
						<div class="settings-field">
							<label>
								<span>Description</span>
								<textarea bind:value={editDescription} rows={3}></textarea>
							</label>
						</div>
						<div class="settings-field">
							<label>
								<span>Definition of done</span>
								<textarea bind:value={editDod} rows={3}></textarea>
							</label>
						</div>
						<div class="settings-field">
							<span class="field-label">Workspace path</span>
							<WorkspacePathField bind:value={editWorkspacePath} />
						</div>
						<button type="submit" class="secondary" disabled={saving}
							>Save changes</button
						>
					</form>
				</section>
			{:else if job.status === 'ongoing'}
				<p class="drawer-note">
					Claimed by session <code>{job.assigned_session_id}</code>.
				</p>
			{/if}

			<section class="drawer-section">
				<h3>Events</h3>
				{#if loadingEvents}
					<p class="drawer-muted">Loading events…</p>
				{:else if events.length === 0}
					<p class="drawer-muted">No events yet.</p>
				{:else}
					<ul class="drawer-events">
						{#each events as ev (ev.id)}
							<li>
								<code>{ev.action}</code>
								<span>{ev.detail}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			{#if startError}
				<p class="drawer-error">{startError}</p>
			{/if}
			{#if openSessionError}
				<p class="drawer-error">{openSessionError}</p>
			{/if}
		{/if}
	</div>

	{#if mode === 'detail' && job}
		<footer class="drawer-footer">
			{#if job.assigned_session_id}
				<button
					type="button"
					class="secondary"
					disabled={openingSession || saving || starting}
					onclick={() => void handleOpenRunSession()}
				>
					<ExternalLink size={14} />
					{openingSession ? 'Opening…' : 'Open run session'}
				</button>
			{/if}
			{#if job.status === 'todo' && !isArchived}
				<button
					type="button"
					class="primary"
					disabled={starting || saving}
					onclick={() => void handleStartInChat()}
				>
					<Play size={14} />
					Start in chat
				</button>
			{/if}
			{#if isBlocked && !isArchived}
				<button
					type="button"
					class="primary"
					disabled={saving || starting || openingSession}
					onclick={() => void onRetry?.(job)}
				>
					<RefreshCw size={14} />
					Retry now
				</button>
			{/if}
			{#if job.status === 'done' && !isArchived}
				<button
					type="button"
					class="secondary"
					disabled={saving || starting || openingSession}
					onclick={() => void onArchive?.(job)}
				>
					<Archive size={14} />
					Archive
				</button>
			{/if}
			{#if isArchived}
				<button
					type="button"
					class="secondary"
					disabled={saving || starting || openingSession}
					onclick={() => void onUnarchive?.(job)}
				>
					<RotateCcw size={14} />
					Unarchive
				</button>
			{/if}
			{#if !isArchived}
				<button
					type="button"
					class="secondary danger"
					disabled={saving || starting || openingSession}
					onclick={() => void onDelete?.(job)}
				>
					<Trash2 size={14} />
					Delete
				</button>
			{/if}
		</footer>
	{/if}
</aside>

<style>
	.drawer-scrim {
		position: fixed;
		inset: 0;
		z-index: 40;
		border: none;
		background: rgba(15, 23, 42, 0.18);
		cursor: pointer;
	}

	.job-drawer {
		position: fixed;
		top: var(--content-panel-inset);
		right: var(--content-panel-inset);
		bottom: var(--content-panel-inset);
		width: min(420px, calc(100vw - 24px));
		z-index: 50;
		display: flex;
		flex-direction: column;
		padding: 0;
		overflow: hidden;
		border-radius: 16px;
		border: 1px solid var(--border-soft);
		background: var(--panel-bg);
		box-shadow: var(--shadow-card);
	}

	.drawer-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		padding: 16px;
		border-bottom: 1px solid var(--border-soft);
		background: var(--panel-bg);
	}

	.drawer-header h2 {
		margin: 0;
		font-size: 16px;
		line-height: 1.35;
		color: var(--text-main);
	}

	.drawer-eyebrow {
		margin: 0 0 4px;
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-muted);
	}

	.drawer-body {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		background: var(--panel-bg);
	}

	.drawer-meta {
		display: grid;
		gap: 8px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.drawer-meta p {
		margin: 0;
		display: grid;
		gap: 2px;
	}

	.drawer-meta span {
		font-size: 11px;
		font-weight: 600;
		color: var(--text-main);
	}

	.drawer-meta code {
		font-size: 11px;
		word-break: break-all;
	}

	.drawer-section h3 {
		margin: 0 0 8px;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.drawer-copy,
	.drawer-pre,
	.drawer-muted,
	.drawer-note {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.drawer-pre {
		white-space: pre-wrap;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid var(--border-soft);
		background: var(--app-bg);
	}

	.drawer-form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.drawer-form textarea {
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 10px;
		font: inherit;
		font-size: 12px;
		background: var(--app-bg);
		color: var(--text-main);
		resize: vertical;
	}

	.drawer-events {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.drawer-events li {
		display: grid;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.drawer-events code {
		font-size: 11px;
	}

	.plan-steps {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.plan-step {
		padding: 8px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--app-bg);
	}

	.plan-step.completed .plan-step-description {
		color: var(--text-muted);
		text-decoration: line-through;
	}

	.plan-step-line {
		display: flex;
		justify-content: space-between;
		gap: 10px;
		align-items: flex-start;
	}

	.plan-step-description {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
	}

	.plan-step-status {
		flex: none;
		padding: 2px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		font-size: 10px;
		font-weight: 700;
		text-transform: capitalize;
		color: var(--text-muted);
	}

	.plan-step-status[data-status='in_progress'] {
		background: color-mix(in srgb, var(--hero-composer-glow-color) 18%, transparent);
		color: var(--text-main);
	}

	.plan-step-status[data-status='completed'] {
		background: color-mix(in srgb, var(--status-success) 14%, transparent);
		color: var(--status-success);
	}

	.plan-step-status[data-status='blocked'] {
		background: color-mix(in srgb, var(--status-error) 12%, transparent);
		color: var(--status-error);
	}

	.drawer-footer {
		display: flex;
		gap: 8px;
		padding: 12px 16px 16px;
		border-top: 1px solid var(--border-soft);
		background: var(--panel-bg);
	}

	.drawer-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error);
	}

	.field-label {
		display: block;
		margin-bottom: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	@media (max-width: 900px) {
		.job-drawer {
			top: 0;
			right: 0;
			bottom: 0;
			width: min(420px, 100vw);
			border-radius: 0;
		}
	}
</style>
