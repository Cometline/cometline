<script lang="ts">
	import type { GroupedJobs, JobColumn } from '$lib/jobs/group-jobs';
	import type { JobResource } from '$lib/client/cometmind';
	import JobsKanbanColumn from './JobsKanbanColumn.svelte';

	let {
		grouped,
		statusFilter = 'all',
		selectedJobId = null,
		onSelectJob,
		onAddJob
	}: {
		grouped: GroupedJobs;
		statusFilter?: 'all' | JobColumn;
		selectedJobId?: string | null;
		onSelectJob: (job: JobResource) => void;
		onAddJob: () => void;
	} = $props();

	const columnMeta: Record<JobColumn, { title: string; showAdd: boolean }> = {
		todo: { title: 'Todo', showAdd: true },
		ongoing: { title: 'Ongoing', showAdd: false },
		done: { title: 'Done', showAdd: false }
	};

	const columns = $derived(
		statusFilter === 'all'
			? (['todo', 'ongoing', 'done'] as const).map((key) => ({
					key,
					title: columnMeta[key].title,
					showAdd: columnMeta[key].showAdd
				}))
			: [
					{
						key: statusFilter,
						title: columnMeta[statusFilter].title,
						showAdd: columnMeta[statusFilter].showAdd
					}
				]
	);
</script>

<div class="kanban-board scrollbar-none" class:single-column={statusFilter !== 'all'}>
	{#each columns as column (column.key)}
		<JobsKanbanColumn
			title={column.title}
			jobs={grouped[column.key]}
			{selectedJobId}
			showAdd={column.showAdd}
			onSelectJob={onSelectJob}
			onAdd={column.showAdd ? onAddJob : undefined}
		/>
	{/each}
</div>

<style>
	.kanban-board {
		display: grid;
		grid-template-columns: repeat(3, minmax(220px, 1fr));
		gap: 12px;
		min-height: 0;
		flex: 1;
		overflow-x: auto;
		padding-bottom: 4px;
	}

	.kanban-board.single-column {
		grid-template-columns: 1fr;
		max-width: 480px;
	}

	@media (max-width: 900px) {
		.kanban-board {
			grid-template-columns: repeat(3, minmax(260px, 1fr));
			scroll-snap-type: x mandatory;
		}

		.kanban-board :global(.kanban-column) {
			scroll-snap-align: start;
		}
	}
</style>
