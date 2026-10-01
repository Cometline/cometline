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
import {
	cacheHitRate,
	formatCacheHit,
	legendRowsForSeries,
	rangeForPreset,
	type RangePreset,
	type UsageGroupBy
} from '$lib/features/usage/format';
import {
	collectAllUsageEvents,
	isCurrentRefresh,
	localTZOffsetMin
} from '$lib/features/usage/load';
import { usageCsvFilename, usageEventsCsv } from '$lib/features/usage/usage-page-csv';

export const USAGE_PAGE_SIZE = 50;
const CSV_LIMIT = 200;

export type UsagePageController = ReturnType<typeof createUsagePageController>;

export function createUsagePageController() {
	let preset = $state<RangePreset>('7d');
	let range = $state(rangeForPreset('7d'));
	let workspaceId = $state('');
	let groupBy = $state<UsageGroupBy>('model');
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

	const legendRows = $derived(
		legendRowsForSeries(
			series?.keys ?? [],
			groupBy === 'kind' ? (summary?.by_kind ?? []) : (summary?.by_model ?? []),
			groupBy
		)
	);
	const legendCosts = $derived(Object.fromEntries(legendRows.map((row) => [row.key, row.cost])));
	const cacheReadTotal = $derived(summary?.totals.cache_read ?? 0);
	const billedInputTotal = $derived(summary?.totals.billed_input ?? 0);
	const cacheHit = $derived(formatCacheHit(cacheHitRate(billedInputTotal, cacheReadTotal)));

	async function refresh(nextOffset = 0) {
		const seq = ++refreshSeq;
		loading = true;
		error = '';
		try {
			const [nextSummary, nextSeries, nextEvents] = await Promise.all([
				getUsageSummary(query),
				getUsageSeries({ ...query, group_by: groupBy, tz_offset_min: localTZOffsetMin() }),
				listUsageEvents({ ...query, limit: USAGE_PAGE_SIZE, offset: nextOffset })
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

	async function exportCsv() {
		try {
			const items = await collectAllUsageEvents(
				(csvOffset, limit) => listUsageEvents({ ...query, limit, offset: csvOffset }),
				CSV_LIMIT
			);
			if (!items.length) return;
			const csv = usageEventsCsv(items);
			const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = usageCsvFilename(range.from, range.to);
			link.click();
			URL.revokeObjectURL(url);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to export usage';
		}
	}

	function load() {
		void listWorkspaces()
			.then((items) => {
				workspaces = items;
			})
			.catch(() => {
				workspaces = [];
			});
		void refresh(0);
	}

	return {
		get preset() {
			return preset;
		},
		get range() {
			return range;
		},
		get workspaceId() {
			return workspaceId;
		},
		set workspaceId(value: string) {
			workspaceId = value;
		},
		get groupBy() {
			return groupBy;
		},
		set groupBy(value: UsageGroupBy) {
			groupBy = value;
		},
		get workspaces() {
			return workspaces;
		},
		get summary() {
			return summary;
		},
		get series() {
			return series;
		},
		get events() {
			return events;
		},
		get offset() {
			return offset;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get legendRows() {
			return legendRows;
		},
		get legendCosts() {
			return legendCosts;
		},
		get cacheReadTotal() {
			return cacheReadTotal;
		},
		get cacheHit() {
			return cacheHit;
		},
		load,
		refresh,
		applyPreset,
		exportCsv
	};
}
