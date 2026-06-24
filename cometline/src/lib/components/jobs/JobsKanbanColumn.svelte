<script lang="ts">
	import { Plus } from '@lucide/svelte';
	import type { JobResource } from '$lib/client/cometmind';
	import JobCard from './JobCard.svelte';

	let {
		title,
		jobs,
		selectedJobId = null,
		showAdd = false,
		onSelectJob,
		onAdd
	}: {
		title: string;
		jobs: JobResource[];
		selectedJobId?: string | null;
		showAdd?: boolean;
		onSelectJob: (job: JobResource) => void;
		onAdd?: () => void;
	} = $props();
</script>

<section class="kanban-column settings-panel-frame">
	<header class="kanban-column-header">
		<div class="kanban-column-heading">
			<h2>{title}</h2>
			<span class="kanban-count">{jobs.length}</span>
		</div>
		{#if showAdd && onAdd}
			<button
				type="button"
				class="kanban-add"
				aria-label="New job"
				title="New job"
				onclick={onAdd}
			>
				<Plus size={14} stroke-width={2} />
			</button>
		{/if}
	</header>

	<div class="kanban-column-body scrollbar-none">
		{#if jobs.length === 0}
			<p class="kanban-empty">No jobs</p>
		{:else}
			{#each jobs as job (job.id)}
				<JobCard
					{job}
					selected={selectedJobId === job.id}
					onclick={() => onSelectJob(job)}
				/>
			{/each}
		{/if}
	</div>
</section>

<style>
	.kanban-column {
		display: flex;
		flex-direction: column;
		min-width: 0;
		min-height: 0;
		height: 100%;
		padding: 12px;
		gap: 10px;
	}

	.kanban-column-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		flex-shrink: 0;
	}

	.kanban-column-heading {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.kanban-column-heading h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
		color: var(--text-main);
	}

	.kanban-count {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.kanban-add {
		width: 28px;
		height: 28px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.82);
		color: var(--text-muted);
		display: grid;
		place-items: center;
		flex-shrink: 0;
		transition:
			background 140ms ease,
			color 140ms ease;
	}

	.kanban-add:hover {
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-main);
	}

	.kanban-column-body {
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-height: 0;
		overflow-y: auto;
		flex: 1;
		padding-right: 2px;
	}

	.kanban-empty {
		margin: 0;
		padding: 12px 4px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
