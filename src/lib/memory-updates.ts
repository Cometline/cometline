import type { MemoryUpdate } from '$lib/types';

export function memoryUpdateHint(updates: MemoryUpdate[]): string {
	if (updates.length === 0) return '';
	if (updates.length === 1 && updates[0]?.action === 'create') return 'Memory saved';
	return 'Memory updated';
}

export function memoryUpdateTooltip(updates: MemoryUpdate[]): string {
	return updates
		.map((update) => {
			const verb =
				update.action === 'create'
					? 'Saved'
					: update.action === 'supersede'
						? 'Replaced with'
						: 'Updated';
			return `${verb} ${update.kind}: ${update.content}`;
		})
		.join('\n');
}
