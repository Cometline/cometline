<script lang="ts">
	import { CircleDollarSign } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import {
		getUsageSeries,
		getUsageSummary,
		listUsageEvents,
		listWorkspaces,
		type UsageEventsResponse,
		type UsageSeriesResponse,
		type UsageSummaryResponse,
		type Workspace
	} from '$lib/client/cometmind';
	import UsageStackedArea from '$lib/components/usage/UsageStackedArea.svelte';
	import { truncateWorkspacePath } from '$lib/jobs/group-jobs';
	import {
		formatEventTime,
		formatKind,
		formatRangeLabel,
		formatTokens,
		formatUSD,
		rangeForPreset,
		type RangePreset
	} from '$lib/usage/format';
	import { collectAllUsageEvents, isCurrentRefresh, localTZOffsetMin } from '$lib/usage/load';

	const PAGE_SIZE = 50;
	const CSV_LIMIT = 200;

	let preset = $state<RangePreset>('7d');
	let range = $state(rangeForPreset('7d'));
	let workspaceId = $state('');
	let groupBy = $state<'model' | 'kind'>('model');
	let workspaces = $state<Workspace[]>([]);
	let summary = $state<UsageSummaryResponse | null>(null);
	let series = $state<UsageSeriesResponse | null>(null);
	let events = $state<UsageEventsResponse | null>(null);
	let offset = $state(0);
	let loading = $state(false);
	let error = $state('');
	let refreshSeq = 0;

	const query = $derived({
		from: range.from,
		to: range.to,
		...(workspaceId ? { workspace_id: workspaceId } : {})
	});

	async function refresh(nextOffset = 0) {
		const seq = ++refreshSeq;
		loading = true;
		error = '';
		try {
			const [nextSummary, nextSeries, nextEvents] = await Promise.all([
				getUsageSummary(query),
				getUsageSeries({ ...query, group_by: groupBy, tz_offset_min: localTZOffsetMin() }),
				listUsageEvents({ ...query, limit: PAGE_SIZE, offset: nextOffset })
			]);
			if (!isCurrentRefresh(seq, refreshSeq)) return;
			summary = nextSummary;
			series = nextSeries;
			events = nextEvents;
			offset = nextOffset;
		} catch (err) {
			if (!isCurrentRefresh(seq, refreshSeq)) return;
			error = err instanceof Error ? err.message : 'Failed to load usage';
		} finally {
			if (isCurrentRefresh(seq, refreshSeq)) loading = false;
		}
	}

	function applyPreset(next: RangePreset) {
		preset = next;
		range = rangeForPreset(next);
		void refresh(0);
	}

	function dateInputValue(ms: number): string {
		const d = new Date(ms);
		const month = String(d.getMonth() + 1).padStart(2, '0');
		const day = String(d.getDate()).padStart(2, '0');
		return `${d.getFullYear()}-${month}-${day}`;
	}

	async function exportCsv() {
		try {
			const items = await collectAllUsageEvents(
				(csvOffset, limit) => listUsageEvents({ ...query, limit, offset: csvOffset }),
				CSV_LIMIT
			);
			if (!items.length) return;
			const rows = [
				['Date', 'Kind', 'Model', 'Tokens', 'Cost'],
				...items.map((item) => [
					new Date(item.created_at).toISOString(),
					item.call_kind,
					item.model_id,
					String(item.input_tokens + item.output_tokens + item.cache_read + item.cache_write),
					item.priced ? item.estimated_usd.toFixed(6) : ''
				])
			];
			const csv = rows.map((row) => row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(',')).join('\n');
			const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = `cometline-usage-${dateInputValue(range.from)}-${dateInputValue(range.to)}.csv`;
			link.click();
			URL.revokeObjectURL(url);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to export usage';
		}
	}

	onMount(() => {
		void listWorkspaces()
			.then((items) => {
				workspaces = items;
			})
			.catch(() => {
				workspaces = [];
			});
		void refresh(0);
	});
</script>

<div class="usage-page settings-ui">
	<p class="usage-desc">Usage is kept for one year. Older events are removed automatically.</p>
	<header class="usage-toolbar">
		<div class="chips" role="group" aria-label="Date range">
			<button type="button" class:active={preset === '7d'} onclick={() => applyPreset('7d')}>Last 7 days</button>
			<button type="button" class:active={preset === '30d'} onclick={() => applyPreset('30d')}>Last 30 days</button>
			<button type="button" class:active={preset === 'month'} onclick={() => applyPreset('month')}>Last month</button>
			<button type="button" class:active={preset === 'year'} onclick={() => applyPreset('year')}>Past year</button>
		</div>
		<div class="usage-controls">
			<p class="usage-range">{formatRangeLabel(range.from, range.to)}</p>
			<label class="field field-workspace">
				<span>Workspace</span>
				<select bind:value={workspaceId} onchange={() => void refresh(0)}>
					<option value="">All workspaces</option>
					{#each workspaces as workspace (workspace.id)}
						<option value={workspace.id}>{truncateWorkspacePath(workspace.path)}</option>
					{/each}
				</select>
			</label>
		</div>
	</header>

	{#if error}
		<p class="usage-error">{error}</p>
	{/if}

	<div class="usage-stack">
	{#if loading && !summary}
		<section class="usage-empty settings-panel-frame" aria-busy="true">
			<p>Loading…</p>
		</section>
	{:else if summary && summary.totals.tokens === 0}
		<section class="usage-empty settings-panel-frame">
			<CircleDollarSign size={28} stroke-width={1.6} />
			<h2>No usage yet</h2>
			<p>Usage is recorded from this version onward.</p>
		</section>
	{:else if summary}
		<section class="kpis">
			<article>
				<strong>{formatTokens(summary.totals.tokens)}</strong>
				<span>Total Tokens</span>
			</article>
			<article>
				<strong>{formatUSD(summary.totals.estimated_usd)}</strong>
				<span>Estimated</span>
			</article>
			<article>
				<strong>{formatTokens(summary.totals.unpriced_tokens)}</strong>
				<span>Unpriced</span>
			</article>
		</section>

		<section class="chart-card settings-panel-frame">
			<div class="chart-head">
				<div>
					<h2>Your usage</h2>
					<p>Cumulative tokens</p>
				</div>
				<label class="field">
					<span>Group by</span>
					<select bind:value={groupBy} onchange={() => void refresh(offset)}>
						<option value="model">Model</option>
						<option value="kind">Kind</option>
					</select>
				</label>
			</div>
			<UsageStackedArea points={series?.points ?? []} keys={series?.keys ?? []} />
			<div class="legend">
				{#each series?.keys ?? [] as key, index (key)}
					<span>
						<i class={`swatch series-${index % 6}`}></i>
						{groupBy === 'kind' ? formatKind(key) : key}
					</span>
				{/each}
			</div>
		</section>

		<section class="table-card settings-panel-frame">
			<div class="table-head">
				<h2>Events</h2>
				<button type="button" onclick={() => void exportCsv()} disabled={!events?.items.length}>Export CSV</button>
			</div>
			<div class="table-wrap">
				<table>
					<thead>
						<tr>
							<th>Date</th>
							<th>Kind</th>
							<th>Model</th>
							<th>Tokens</th>
							<th>Cost</th>
						</tr>
					</thead>
					<tbody>
						{#each events?.items ?? [] as item (item.id)}
							<tr>
								<td>{formatEventTime(item.created_at)}</td>
								<td>{formatKind(item.call_kind)}</td>
								<td>{item.model_id}</td>
								<td>{formatTokens(item.input_tokens + item.output_tokens + item.cache_read + item.cache_write)}</td>
								<td>{item.priced ? formatUSD(item.estimated_usd) : '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			<div class="pager">
				<span>Rows {PAGE_SIZE} · Showing {events ? `${offset + 1}–${offset + (events.items.length || 0)} of ${events.total}` : '0'}</span>
				<div>
					<button type="button" disabled={offset <= 0} onclick={() => void refresh(Math.max(0, offset - PAGE_SIZE))}>Prev</button>
					<button
						type="button"
						disabled={!events || offset + events.items.length >= events.total}
						onclick={() => void refresh(offset + PAGE_SIZE)}>Next</button>
				</div>
			</div>
		</section>
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

	.usage-toolbar,
	.usage-stack,
	.usage-desc,
	.usage-error,
	.kpis,
	.chart-card,
	.table-card,
	.usage-empty {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
	}

	.usage-toolbar {
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

	.chart-card .field {
		flex-direction: row;
		align-items: center;
	}

	.chart-card .field select {
		width: 7.5rem;
	}

	.chips button,
	.table-head button,
	.pager button {
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

	.kpis {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 12px;
	}

	.kpis article {
		min-width: 0;
		padding: 16px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: var(--panel-bg);
	}

	.kpis strong {
		display: block;
		font-size: 28px;
		letter-spacing: -0.03em;
	}

	.kpis span {
		color: var(--text-muted);
		font-size: 12px;
	}

	.chart-card,
	.table-card {
		display: grid;
		gap: 12px;
	}

	.chart-head,
	.table-head,
	.pager {
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
		display: flex;
		flex-wrap: wrap;
		gap: 10px 16px;
		color: var(--text-muted);
		font-size: 12px;
	}

	.swatch {
		display: inline-block;
		width: 8px;
		height: 8px;
		margin-right: 6px;
		border-radius: 99px;
	}

	.series-0 {
		background: var(--status-success);
	}
	.series-1 {
		background: var(--intro-blue);
	}
	.series-2 {
		background: var(--accent);
	}
	.series-3 {
		background: var(--status-warning);
	}
	.series-4 {
		background: var(--text-soft);
	}
	.series-5 {
		background: var(--status-error);
	}

	.table-wrap {
		overflow: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 12px;
	}

	th,
	td {
		text-align: left;
		padding: 8px 6px;
		border-bottom: 1px solid var(--border-soft);
	}

	th {
		color: var(--text-muted);
		font-weight: 550;
	}

	.pager {
		color: var(--text-muted);
		font-size: 12px;
	}

	@container main-pane (max-width: 760px) {
		.usage-page {
			padding: 16px;
		}

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

		.kpis {
			gap: 8px;
		}

		.kpis article {
			padding: 12px;
		}

		.kpis strong {
			font-size: 22px;
		}
	}
</style>
