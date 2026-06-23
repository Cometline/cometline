<script lang="ts">
	import { fly, fade } from 'svelte/transition';
	import { onMount } from 'svelte';
	import { X, Trash2, Play } from '@lucide/svelte';
	import type { JobEventResource, JobResource } from '$lib/client/cometmind';
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
		editDescription = $bindable(''),
		editDod = $bindable(''),
		editWorkspacePath = $bindable(''),
		createDescription = $bindable(''),
		createDod = $bindable(''),
		createWorkspacePath = $bindable(''),
		onClose,
		onSave,
		onDelete,
		onCreate,
		onStartInChat
	}: {
		job?: JobResource | null;
		mode?: DrawerMode;
		events?: JobEventResource[];
		saving?: boolean;
		loadingEvents?: boolean;
		editDescription?: string;
		editDod?: string;
		editWorkspacePath?: string;
		createDescription?: string;
		createDod?: string;
		createWorkspacePath?: string;
		onClose: () => void;
		onSave?: () => void | Promise<void>;
		onDelete?: (job: JobResource) => void | Promise<void>;
		onCreate?: () => void | Promise<void>;
		onStartInChat?: (job: JobResource) => void | Promise<void>;
	} = $props();

	let starting = $state(false);
	let startError = $state('');
	const isArchived = $derived(job?.deleted_at != null);

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
</script>

<button class="drawer-scrim" aria-label="Close job details" onclick={onClose} transition:fade={{ duration: 120 }}
></button>

<aside class="job-drawer settings-ui" transition:fly={{ x: 320, duration: 200 }}>
	<header class="drawer-header">
		<div>
			<p class="drawer-eyebrow">{mode === 'create' ? 'New job' : job?.status ?? ''}</p>
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
				{#if job.assigned_session_id}
					<p><span>Assigned</span> <code>{job.assigned_session_id}</code></p>
				{/if}
				{#if job.workspace_path}
					<p><span>Workspace</span> <code>{job.workspace_path}</code></p>
				{/if}
			</div>

			{#if job.progress?.trim()}
				<section class="drawer-section">
					<h3>Progress</h3>
					<pre class="drawer-pre">{job.progress}</pre>
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
						<button type="submit" class="secondary" disabled={saving}>Save changes</button>
					</form>
				</section>
			{:else if job.status === 'ongoing'}
				<p class="drawer-note">Claimed by session <code>{job.assigned_session_id}</code>.</p>
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
		{/if}
	</div>

	{#if mode === 'detail' && job && !isArchived}
		<footer class="drawer-footer">
			{#if job.status === 'todo'}
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
			<button
				type="button"
				class="secondary danger"
				disabled={saving || starting}
				onclick={() => void onDelete?.(job)}
			>
				<Trash2 size={14} />
				Delete
			</button>
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
		color: #b42318;
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
