<script lang="ts">
	import { truncateWorkspacePath } from '#lib/features/jobs/group-jobs.js';
	import { formatRangeLabel, type RangePreset } from '#lib/features/usage/format.js';
	import type { UsagePageController } from '#lib/features/usage/usage-page-controller.svelte.js';

	let { controller }: { controller: UsagePageController } = $props();

	const presets: { id: RangePreset; label: string }[] = [
		{ id: 'today', label: 'Today' },
		{ id: '7d', label: 'Last 7 days' },
		{ id: '30d', label: 'Last 30 days' },
		{ id: 'month', label: 'Last month' },
		{ id: 'year', label: 'Past year' }
	];
</script>

<header class="usage-toolbar">
	<div class="chips" role="group" aria-label="Date range">
		{#each presets as item (item.id)}
			<button
				type="button"
				class:active={controller.preset === item.id}
				onclick={() => controller.applyPreset(item.id)}>{item.label}</button
			>
		{/each}
	</div>
	<div class="usage-controls">
		<p class="usage-range">{formatRangeLabel(controller.range.from, controller.range.to)}</p>
		<label class="field field-workspace">
			<span>Workspace</span>
			<select bind:value={controller.workspaceId} onchange={() => void controller.refresh(0)}>
				<option value="">All workspaces</option>
				{#each controller.workspaces as workspace (workspace.id)}
					<option value={workspace.id}>{truncateWorkspacePath(workspace.path)}</option>
				{/each}
			</select>
		</label>
	</div>
</header>

<style>
	.usage-toolbar {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 10px 16px;
		flex-shrink: 0;
	}

	.usage-controls {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: flex-end;
		gap: 8px 12px;
		min-width: 0;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.field {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		margin: 0;
		font-size: 11px;
		color: var(--text-muted);
	}

	.field > span {
		flex-shrink: 0;
	}

	.usage-range {
		margin: 0;
		color: var(--text-muted);
		font-size: 12px;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.field-workspace select {
		width: 11.5rem;
	}

	.chips button {
		border: 1px solid var(--border-soft);
		background: var(--panel-bg);
		color: var(--text-main);
		border-radius: 8px;
		padding: 6px 10px;
		font-size: 12px;
	}

	.chips button.active {
		border-color: var(--accent);
	}

	select {
		box-sizing: border-box;
		max-width: 100%;
		border: 1px solid var(--border-soft);
		background: var(--panel-bg);
		color: var(--text-main);
		border-radius: 8px;
		padding: 6px 8px;
		font-size: 12px;
	}

	@container main-pane (max-width: 760px) {
		.usage-toolbar,
		.usage-controls {
			align-items: stretch;
		}

		.usage-controls {
			display: flex;
			flex-direction: column;
			align-items: stretch;
			width: 100%;
		}

		.usage-range,
		.field-workspace,
		.field-workspace select {
			width: 100%;
		}
	}
</style>
