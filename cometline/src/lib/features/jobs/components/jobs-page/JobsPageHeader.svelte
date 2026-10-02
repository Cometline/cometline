<script lang="ts">
	import { RefreshCw } from '@lucide/svelte';
	import type { JobsStatusFilter, JobsView } from '#lib/features/jobs/jobs-page.svelte.js';

	const STATUS_FILTERS: { id: JobsStatusFilter; label: string }[] = [
		{ id: 'all', label: 'All' },
		{ id: 'todo', label: 'Todo' },
		{ id: 'ongoing', label: 'Ongoing' },
		{ id: 'done', label: 'Done' }
	];

	let {
		view = $bindable(),
		statusFilter = $bindable(),
		loading,
		refreshing,
		onRefresh
	}: {
		view: JobsView;
		statusFilter: JobsStatusFilter;
		loading: boolean;
		refreshing: boolean;
		onRefresh: () => void;
	} = $props();
</script>

<header class="jobs-header">
	<p>Global work queue shared across sessions.</p>
	<div class="jobs-header-actions">
		{#if view === 'active'}
			<div class="status-filters" role="group" aria-label="Filter by status">
				{#each STATUS_FILTERS as filter (filter.id)}
					<button
						type="button"
						class="status-filter"
						class:active={statusFilter === filter.id}
						aria-pressed={statusFilter === filter.id}
						onclick={() => (statusFilter = filter.id)}
					>
						{filter.label}
					</button>
				{/each}
			</div>
		{/if}
		<div class="view-toggle" role="group" aria-label="Switch view">
			<button
				type="button"
				class="view-btn"
				class:active={view === 'active'}
				aria-pressed={view === 'active'}
				onclick={() => (view = 'active')}
			>
				Active
			</button>
			<button
				type="button"
				class="view-btn"
				class:active={view === 'archived'}
				aria-pressed={view === 'archived'}
				onclick={() => (view = 'archived')}
			>
				Archived
			</button>
			<button
				type="button"
				class="view-btn"
				class:active={view === 'scheduled'}
				aria-pressed={view === 'scheduled'}
				onclick={() => (view = 'scheduled')}
			>
				Scheduled
			</button>
		</div>
		<button
			type="button"
			class="secondary icon-only"
			aria-label="Refresh jobs"
			title="Refresh"
			disabled={loading || refreshing}
			onclick={onRefresh}
		>
			<RefreshCw size={14} class={refreshing ? 'spin' : ''} />
		</button>
	</div>
</header>

<style>
	.jobs-header {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px 16px;
		width: 100%;
		min-width: 0;
		flex-shrink: 0;
		margin-bottom: 16px;
	}

	.jobs-header > p {
		flex: 1;
		min-width: 0;
		margin: 0;
		padding-left: 14px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.jobs-header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
		flex-wrap: wrap;
		justify-content: flex-end;
	}

	.status-filters {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.05);
	}

	.status-filter {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		font-weight: 600;
		padding: 5px 10px;
		border-radius: 999px;
		cursor: pointer;
	}

	.status-filter.active {
		background: var(--panel-bg);
		color: var(--text-main);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
	}

	.status-filter:hover:not(.active) {
		color: var(--text-main);
	}

	.view-toggle {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.05);
	}

	.view-btn {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		font-weight: 600;
		padding: 5px 10px;
		border-radius: 999px;
		cursor: pointer;
	}

	.view-btn.active {
		background: var(--panel-bg);
		color: var(--text-main);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
	}

	.view-btn:hover:not(.active) {
		color: var(--text-main);
	}

	@media (max-width: 900px) {
		.jobs-header {
			flex-direction: column;
		}
	}

	@container main-pane (max-width: 760px) {
		.jobs-header {
			flex-direction: column;
			align-items: flex-start;
		}

		.jobs-header-actions {
			width: 100%;
			justify-content: flex-start;
		}
	}
</style>
