<script lang="ts">
	import type { JobResource } from '$lib/client/cometmind';
	import JobCard from '../JobCard.svelte';

	let {
		archivedJobs,
		selectedJobId,
		onSelectJob
	}: {
		archivedJobs: JobResource[];
		selectedJobId: string | null;
		onSelectJob: (job: JobResource) => void;
	} = $props();
</script>

<section class="archived-panel settings-panel-frame">
	<header class="archived-header">
		<h2>Archived</h2>
		<span class="archived-count">{archivedJobs.length}</span>
	</header>
	{#if archivedJobs.length === 0}
		<p class="jobs-muted">No archived jobs.</p>
	{:else}
		<div class="archived-list scrollbar-none">
			{#each archivedJobs as job (job.id)}
				<JobCard
					{job}
					selected={selectedJobId === job.id}
					onclick={() => onSelectJob(job)}
				/>
			{/each}
		</div>
	{/if}
</section>

<style>
	.jobs-muted {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.archived-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		height: 100%;
		padding: 14px;
		overflow: hidden;
	}

	.archived-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.archived-header h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
	}

	.archived-count {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.archived-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 10px;
		overflow-y: auto;
		min-height: 0;
		padding-right: 2px;
	}
</style>
