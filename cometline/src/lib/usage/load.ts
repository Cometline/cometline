export type UsageEventPage<T> = {
	items: T[];
	total: number;
};

export function isCurrentRefresh(seq: number, current: number): boolean {
	return seq === current;
}

export async function collectAllUsageEvents<T>(
	listPage: (offset: number, limit: number) => Promise<UsageEventPage<T>>,
	pageSize = 200
): Promise<T[]> {
	const items: T[] = [];
	let offset = 0;
	let total = Number.POSITIVE_INFINITY;
	while (items.length < total) {
		const page = await listPage(offset, pageSize);
		total = page.total;
		if (!page.items.length) break;
		items.push(...page.items);
		offset += page.items.length;
	}
	return items;
}

export function localTZOffsetMin(now = new Date()): number {
	return -now.getTimezoneOffset();
}
