<script lang="ts">
	import { CircleDollarSign } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import {
		createUsagePageController,
		USAGE_PAGE_SIZE
	} from '#lib/features/usage/usage-page-controller.svelte.js';
	import UsageChartCard from './usage-page/UsageChartCard.svelte';
	import UsageEventsTable from './usage-page/UsageEventsTable.svelte';
	import UsageKpis from './usage-page/UsageKpis.svelte';
	import UsageToolbar from './usage-page/UsageToolbar.svelte';

	const controller = createUsagePageController();

	onMount(() => {
		controller.load();
	});
</script>

<div class="usage-page settings-ui">
	<p class="usage-desc">
		Usage is kept for one year. Estimated cost uses public API rates. Cached input is billed at
		the cache rate when the provider reports it.
	</p>
	<UsageToolbar {controller} />

	{#if controller.error}
		<p class="usage-error">{controller.error}</p>
	{/if}

	<div class="usage-stack">
		{#if controller.loading && !controller.summary}
			<section class="usage-empty settings-panel-frame" aria-busy="true">
				<p>Loading…</p>
			</section>
		{:else if controller.summary && controller.summary.totals.tokens === 0}
			<section class="usage-empty settings-panel-frame">
				<CircleDollarSign size={28} stroke-width={1.6} />
				<h2>No usage yet</h2>
				<p>Usage is recorded from this version onward.</p>
			</section>
		{:else if controller.summary}
			<UsageKpis
				totals={controller.summary.totals}
				cacheReadTotal={controller.cacheReadTotal}
				cacheHit={controller.cacheHit}
			/>
			<UsageChartCard {controller} />
			<UsageEventsTable
				events={controller.events}
				offset={controller.offset}
				pageSize={USAGE_PAGE_SIZE}
				onExport={() => void controller.exportCsv()}
				onPage={(nextOffset) => void controller.refresh(nextOffset)}
			/>
		{/if}
	</div>
</div>

<style>
	.usage-page {
		display: flex;
		flex-direction: column;
		box-sizing: border-box;
		width: 100%;
		max-width: 100%;
		min-width: 0;
		height: 100%;
		min-height: 0;
		gap: 16px;
		padding: 20px 24px;
		overflow: auto;
	}

	.usage-stack,
	.usage-desc,
	.usage-error,
	.usage-empty {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
	}

	.usage-desc,
	.usage-error {
		margin: 0;
		padding-left: 14px;
		font-size: 12px;
		line-height: 1.5;
	}

	.usage-desc {
		color: var(--text-muted);
	}

	.usage-error {
		color: var(--status-error);
	}

	.usage-stack {
		display: flex;
		flex-direction: column;
		flex: 1;
		gap: 16px;
		min-height: 0;
	}

	.usage-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		text-align: center;
		color: var(--text-muted);
	}

	.usage-empty h2 {
		margin: 0;
		font-size: 15px;
		color: var(--text-main);
	}

	@container main-pane (max-width: 760px) {
		.usage-page {
			padding: 16px;
		}
	}
</style>
