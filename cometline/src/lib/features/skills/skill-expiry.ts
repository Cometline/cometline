const dayMs = 24 * 60 * 60 * 1000;

export function skillExpiryLabel(
	expiresAt: number | undefined,
	status: string | undefined,
	now = Date.now(),
	pinned = false
): string {
	if (!expiresAt || pinned) return '';
	const days = Math.ceil((expiresAt - now) / dayMs);
	const verb = status === 'archived' ? 'deletes' : 'archives';
	if (days <= 0) return `${verb} today`;
	if (days === 1) return `${verb} in 1 day`;
	return `${verb} in ${days} days`;
}
