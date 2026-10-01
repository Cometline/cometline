export function formatRelativeTime(ms: number, now = Date.now()): string {
	const delta = now - ms;
	const minutes = Math.floor(delta / 60_000);
	if (minutes < 1) return 'just now';
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	return `${days}d ago`;
}

export function previewSnippet(body: string, max = 96): string {
	const compact = body.replace(/\s+/g, ' ').trim();
	if (compact.length <= max) return compact;
	return `${compact.slice(0, max - 1)}…`;
}
