<script lang="ts">
	import UsageStackedArea from '#lib/features/usage/components/UsageStackedArea.svelte';
	import { seriesColor } from '#lib/features/usage/chart.js';
	import { formatTokens } from '#lib/features/usage/format.js';
	import type { UsagePageController } from '#lib/features/usage/usage-page-controller.svelte.js';

	let { controller }: { controller: UsagePageController } = $props();
</script>

<section class="chart-card settings-panel-frame">
	<div class="chart-head">
		<div>
			<h2>Your usage</h2>
			<p>Cumulative tokens</p>
		</div>
		<label class="field">
			<span>Group by</span>
			<select
				bind:value={controller.groupBy}
				onchange={() => void controller.refresh(controller.offset)}
			>
				<option value="model">Model</option>
				<option value="kind">Kind</option>
			</select>
		</label>
	</div>
	<UsageStackedArea
		points={controller.series?.points ?? []}
		keys={controller.series?.keys ?? []}
		costs={controller.legendCosts}
	/>
	<div class="legend">
		<div class="legend-head">
			<span>{controller.groupBy === 'kind' ? 'Kind' : 'Model'}</span>
			<span>Tokens</span>
			<span>Cache</span>
			<span>Cost</span>
		</div>
		{#each controller.legendRows as row, index (row.key)}
			<div class="legend-row">
				<span class="legend-name">
					<i
						class="swatch"
						style:background={seriesColor(index, controller.legendRows.length)}
					></i>
					{row.label}
				</span>
				<span class="legend-meta">{formatTokens(row.tokens)}</span>
				<span class="legend-meta">{row.cache}</span>
				<span class="legend-meta">{row.cost}</span>
			</div>
		{/each}
	</div>
</section>

<style>
	.chart-card {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		display: grid;
		gap: 12px;
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

	.chart-card .field {
		flex-direction: row;
		align-items: center;
	}

	.chart-card .field select {
		width: 7.5rem;
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

	.chart-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		min-width: 0;
	}

	h2 {
		margin: 0;
		font-size: 15px;
	}

	.chart-head p {
		margin: 2px 0 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.legend {
		display: grid;
		gap: 6px 16px;
		color: var(--text-muted);
		font-size: 12px;
	}

	.legend-head,
	.legend-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 4.5rem 4.5rem 4.5rem;
		gap: 12px;
		align-items: center;
	}

	.legend-head {
		color: var(--text-muted);
		font-weight: 550;
	}

	.legend-head span:not(:first-child),
	.legend-meta {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}

	.legend-name {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-main);
	}

	.swatch {
		display: inline-block;
		width: 8px;
		height: 8px;
		margin-right: 6px;
		border-radius: 99px;
	}
</style>
