export function formatTokens(n: number): string {
	if (!Number.isFinite(n) || n === 0) return '0';
	const abs = Math.abs(n);
	if (abs >= 1_000_000) return `${trimFloat(n / 1_000_000)}M`;
	if (abs >= 1_000) return `${trimFloat(n / 1_000)}k`;
	return String(Math.round(n));
}

export function formatUSD(n: number): string {
	if (!Number.isFinite(n) || n === 0) return '$0.00';
	if (Math.abs(n) < 0.01) return `$${n.toFixed(4)}`;
	return `$${n.toFixed(2)}`;
}

export function formatKind(kind: string): string {
	switch (kind) {
		case 'agent_step':
			return 'Agent';
		case 'memory_extract':
			return 'Memory extract';
		case 'memory_update':
			return 'Memory update';
		case 'memory_compaction':
			return 'Memory compaction';
		case 'embedding':
			return 'Embedding';
		case 'skill_synthesis':
			return 'Skill synthesis';
		default:
			return kind || 'Unknown';
	}
}

export function formatEventTime(ms: number): string {
	return new Date(ms).toLocaleString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: 'numeric',
		minute: '2-digit'
	});
}

function trimFloat(n: number): string {
	return n.toFixed(1).replace(/\.0$/, '');
}

export type RangePreset = 'today' | '7d' | '30d' | 'month' | 'year';
export type UsageGroupBy = 'model' | 'kind';

export type UsageSeriesBucket = {
	key: string;
	provider_id?: string;
	model_id?: string;
	call_kind?: string;
	tokens: number;
	estimated_usd: number;
	priced: boolean;
};

export const MAX_RANGE_DAYS = 366;

export function formatRangeLabel(from: number, to: number): string {
	const start = new Date(from);
	const end = new Date(to - 1);
	const opts: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric', year: 'numeric' };
	const startLabel = start.toLocaleDateString(undefined, opts);
	const endLabel = end.toLocaleDateString(undefined, opts);
	if (startLabel === endLabel) return startLabel;
	return `${startLabel} – ${endLabel}`;
}

export function clampUsageRange(from: number, to: number, now = new Date()): { from: number; to: number } {
	const maxMs = MAX_RANGE_DAYS * 24 * 60 * 60 * 1000;
	const latest = now.getTime() + 1;
	if (!Number.isFinite(to) || to > latest) to = latest;
	if (!Number.isFinite(from) || from < 0) from = 0;
	const earliest = latest - maxMs;
	if (from < earliest) from = earliest;
	if (to <= from) to = Math.min(latest, from + 24 * 60 * 60 * 1000);
	if (to - from > maxMs) from = to - maxMs;
	return { from, to };
}

export function rangeForPreset(preset: RangePreset, now = new Date()): { from: number; to: number } {
	const to = now.getTime() + 1;
	if (preset === 'today') {
		const from = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
		return { from, to };
	}
	if (preset === 'month') {
		const start = new Date(now.getFullYear(), now.getMonth() - 1, 1);
		const end = new Date(now.getFullYear(), now.getMonth(), 1);
		return { from: start.getTime(), to: end.getTime() };
	}
	if (preset === 'year') {
		return clampUsageRange(0, to, now);
	}
	const days = preset === '30d' ? 30 : 7;
	const from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - (days - 1)).getTime();
	return { from, to };
}

export function seriesKeyForBucket(bucket: UsageSeriesBucket, groupBy: UsageGroupBy): string {
	if (groupBy === 'kind') {
		return bucket.call_kind || bucket.key;
	}
	const provider = bucket.provider_id?.trim() ?? '';
	const model = (bucket.model_id || bucket.key).trim();
	if (!provider) return model;
	if (!model) return provider;
	return `${provider}/${model}`;
}

export function indexSeriesBuckets(
	buckets: UsageSeriesBucket[],
	groupBy: UsageGroupBy
): Record<string, UsageSeriesBucket> {
	const indexed: Record<string, UsageSeriesBucket> = {};
	for (const bucket of buckets) {
		indexed[seriesKeyForBucket(bucket, groupBy)] = bucket;
	}
	return indexed;
}

export function formatBucketCost(bucket?: Pick<UsageSeriesBucket, 'priced' | 'estimated_usd'>): string {
	if (!bucket?.priced) return '—';
	return formatUSD(bucket.estimated_usd);
}

export type UsageLegendRow = {
	key: string;
	label: string;
	tokens: number;
	cost: string;
};

export function legendRowsForSeries(
	keys: string[],
	buckets: UsageSeriesBucket[],
	groupBy: UsageGroupBy
): UsageLegendRow[] {
	const indexed = indexSeriesBuckets(buckets, groupBy);
	return keys.map((key) => {
		const bucket = indexed[key];
		return {
			key,
			label: groupBy === 'kind' ? formatKind(key) : key,
			tokens: bucket?.tokens ?? 0,
			cost: formatBucketCost(bucket)
		};
	});
}
