<script lang="ts">
	import type { JobResource } from '$lib/client/cometmind';
	import {
		formatClock,
		formatRelativeTime,
		leaseLabel,
		progressPreview,
		sessionLabel
	} from '$lib/features/jobs/jobs-page-format';
	import { OBSERVER_REFRESH_MS } from '$lib/features/jobs/jobs-page.svelte';

	let {
		activeJobs,
		readyJobCount,
		lastLoadedAt,
		refreshing,
		nowMs,
		selectedJobId,
		onSelectJob
	}: {
		activeJobs: JobResource[];
		readyJobCount: number;
		lastLoadedAt: number;
		refreshing: boolean;
		nowMs: number;
		selectedJobId: string | null;
		onSelectJob: (job: JobResource) => void;
	} = $props();
</script>

<section class="autonomy-observer settings-panel-frame" aria-label="Live job activity">
	<header class="observer-header">
		<div>
			<p class="observer-eyebrow">Live activity</p>
			<h2>Autonomous job observation</h2>
		</div>
		<div class="observer-status">
			<span>{activeJobs.length} running</span>
			<span>{readyJobCount} ready</span>
			<span>Updated {formatClock(lastLoadedAt)}</span>
			{#if refreshing}
				<span class="observer-polling">Refreshing…</span>
			{/if}
		</div>
	</header>

	{#if activeJobs.length === 0}
		<p class="observer-empty">
			No jobs are running. When autonomous pickup or a chat session claims a ready job, it
			will appear here within {Math.round(OBSERVER_REFRESH_MS / 1_000)} seconds.
		</p>
	{:else}
		<div class="observer-list">
			{#each activeJobs as job (job.id)}
				<button
					type="button"
					class="observer-job"
					class:selected={selectedJobId === job.id}
					onclick={() => onSelectJob(job)}
				>
					<div class="observer-job-main">
						<span class="observer-dot" aria-hidden="true"></span>
						<div>
							<strong>{job.description}</strong>
							<p>{progressPreview(job) || 'No progress note yet.'}</p>
						</div>
					</div>
					<div class="observer-job-meta">
						<span>Session {sessionLabel(job)}</span>
						<span>{leaseLabel(job, nowMs)}</span>
						<span>Updated {formatRelativeTime(nowMs, job.updated_at)}</span>
					</div>
				</button>
			{/each}
		</div>
	{/if}
</section>

<style>
	.autonomy-observer {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 14px;
		flex-shrink: 0;
		background:
			linear-gradient(135deg, rgba(96, 165, 250, 0.1), rgba(168, 85, 247, 0.08)),
			var(--panel-bg);
	}

	.observer-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
	}

	.observer-eyebrow {
		margin: 0 0 3px;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-muted);
	}

	.observer-header h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
		color: var(--text-main);
	}

	.observer-status,
	.observer-job-meta {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 6px;
	}

	.observer-status span,
	.observer-job-meta span {
		font-size: 10px;
		font-weight: 600;
		line-height: 1.3;
		padding: 3px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.observer-status .observer-polling {
		color: var(--accent);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}

	.observer-empty {
		margin: 0;
		max-width: 760px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.observer-list {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
		gap: 10px;
	}

	.observer-job {
		width: 100%;
		border: 1px solid color-mix(in srgb, var(--accent) 18%, var(--border-soft));
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.78);
		padding: 10px 12px;
		text-align: left;
		cursor: pointer;
		display: flex;
		flex-direction: column;
		gap: 9px;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.observer-job:hover,
	.observer-job.selected {
		background: rgba(255, 255, 255, 0.96);
		border-color: var(--pane-focus-border);
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
	}

	.observer-job-main {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 8px;
		align-items: flex-start;
	}

	.observer-dot {
		width: 8px;
		height: 8px;
		margin-top: 5px;
		border-radius: 999px;
		background: var(--accent);
		box-shadow: 0 0 0 5px color-mix(in srgb, var(--accent) 14%, transparent);
	}

	.observer-job strong {
		display: block;
		font-size: 12px;
		line-height: 1.35;
		color: var(--text-main);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.observer-job p {
		margin: 3px 0 0;
		font-size: 11px;
		line-height: 1.4;
		color: var(--text-muted);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	@media (max-width: 900px) {
		.observer-header {
			flex-direction: column;
		}
	}

	@container main-pane (max-width: 760px) {
		.observer-header {
			flex-direction: column;
			align-items: flex-start;
		}
	}
</style>
