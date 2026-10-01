export type UsageCsvEvent = {
	created_at: number;
	call_kind: string;
	model_id: string;
	input_tokens: number;
	cache_read: number;
	cache_write: number;
	output_tokens: number;
	priced: boolean;
	estimated_usd: number;
};

export function usageEventsCsv(items: UsageCsvEvent[]): string {
	const rows = [
		['Date', 'Kind', 'Model', 'Input', 'Cache read', 'Cache write', 'Output', 'Cost'],
		...items.map((item) => [
			new Date(item.created_at).toISOString(),
			item.call_kind,
			item.model_id,
			String(item.input_tokens),
			String(item.cache_read),
			String(item.cache_write),
			String(item.output_tokens),
			item.priced ? item.estimated_usd.toFixed(6) : ''
		])
	];
	return rows
		.map((row) => row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(','))
		.join('\n');
}

export function dateInputValue(ms: number): string {
	const d = new Date(ms);
	const month = String(d.getMonth() + 1).padStart(2, '0');
	const day = String(d.getDate()).padStart(2, '0');
	return `${d.getFullYear()}-${month}-${day}`;
}

export function usageCsvFilename(from: number, to: number): string {
	return `cometline-usage-${dateInputValue(from)}-${dateInputValue(to)}.csv`;
}
