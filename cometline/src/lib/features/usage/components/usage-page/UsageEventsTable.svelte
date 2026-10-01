<script lang="ts">
	import type { UsageEventsResponse } from '$lib/client/cometmind';
	import {
		formatEventTime,
		formatKind,
		formatTokens,
		formatUSD
	} from '$lib/features/usage/format';

	let {
		events,
		offset,
		pageSize,
		onExport,
		onPage
	}: {
		events: UsageEventsResponse | null;
		offset: number;
		pageSize: number;
		onExport: () => void;
		onPage: (offset: number) => void;
	} = $props();
</script>

<section class="table-card settings-panel-frame">
	<div class="table-head">
		<h2>Events</h2>
		<button type="button" onclick={onExport} disabled={!events?.items.length}>Export CSV</button
		>
	</div>
	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					<th>Date</th>
					<th>Kind</th>
					<th>Model</th>
					<th>Input</th>
					<th>Cache</th>
					<th>Output</th>
					<th>Cost</th>
				</tr>
			</thead>
			<tbody>
				{#each events?.items ?? [] as item (item.id)}
					<tr>
						<td>{formatEventTime(item.created_at)}</td>
						<td>{formatKind(item.call_kind)}</td>
						<td>{item.model_id}</td>
						<td>{formatTokens(item.billed_input)}</td>
						<td>{item.cache_read > 0 ? formatTokens(item.cache_read) : '—'}</td>
						<td>{formatTokens(item.output_tokens)}</td>
						<td>{item.priced ? formatUSD(item.estimated_usd) : '—'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<div class="pager">
		<span
			>Rows {pageSize} · Showing {events
				? `${offset + 1}–${offset + (events.items.length || 0)} of ${events.total}`
				: '0'}</span
		>
		<div>
			<button
				type="button"
				disabled={offset <= 0}
				onclick={() => onPage(Math.max(0, offset - pageSize))}>Prev</button
			>
			<button
				type="button"
				disabled={!events || offset + events.items.length >= events.total}
				onclick={() => onPage(offset + pageSize)}>Next</button
			>
		</div>
	</div>
</section>

<style>
	.table-card {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		display: grid;
		gap: 12px;
	}

	.table-head button,
	.pager button {
		border: 1px solid var(--border-soft);
		background: var(--panel-bg);
		color: var(--text-main);
		border-radius: 8px;
		padding: 6px 10px;
		font-size: 12px;
	}

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
</style>
