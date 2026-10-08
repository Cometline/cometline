function toggleExpanded(set: Set<string>, id: string) {
	const next = new Set(set);
	if (next.has(id)) next.delete(id);
	else next.add(id);
	return next;
}

function toggleMapOverride(map: Map<string, boolean>, id: string, current: boolean) {
	const next = new Map(map);
	next.set(id, !current);
	return next;
}

export { toggleExpanded, toggleMapOverride };
