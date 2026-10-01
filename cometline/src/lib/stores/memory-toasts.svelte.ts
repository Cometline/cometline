import type {
	MemoryChangeWire,
	MemoryCompactionCompletedEvent
} from '$lib/generated/cometmind-api';

export type MemoryToastAction = MemoryChangeWire['action'] | 'compact';

export interface MemoryToast {
	id: string;
	action: MemoryToastAction;
	label: string;
	preview: string;
}

const TOAST_DURATION_MS = 5000;
const MAX_TOASTS = 4;

const LABELS: Record<MemoryToastAction, string> = {
	create: 'Memory saved',
	update: 'Memory updated',
	delete: 'Memory removed',
	supersede: 'Memory updated',
	compact: 'Memory compaction complete'
};

function previewContent(content: string) {
	const compact = content.replace(/\s+/g, ' ').trim();
	if (compact.length <= 96) return compact;
	return `${compact.slice(0, 93)}…`;
}

function createMemoryToastStore() {
	let toasts = $state<MemoryToast[]>([]);
	const timers = new Map<string, ReturnType<typeof setTimeout>>();

	function dismiss(id: string) {
		const timer = timers.get(id);
		if (timer) clearTimeout(timer);
		timers.delete(id);
		toasts = toasts.filter((toast) => toast.id !== id);
	}

	function add(changes: MemoryChangeWire[]) {
		for (const change of changes) {
			const toast: MemoryToast = {
				id: `memory-toast-${Date.now()}-${Math.random().toString(36).slice(2)}`,
				action: change.action,
				label: LABELS[change.action],
				preview: previewContent(change.content)
			};
			toasts = [...toasts, toast].slice(-MAX_TOASTS);
			timers.set(
				toast.id,
				setTimeout(() => dismiss(toast.id), TOAST_DURATION_MS)
			);
		}
	}

	function addCompaction(event: MemoryCompactionCompletedEvent) {
		const removed = Math.max(0, event.before - event.after);
		const preview =
			removed === 0
				? `${event.after} memories · no changes`
				: `${event.before} → ${event.after} memories · ${removed} removed`;
		const toast: MemoryToast = {
			id: `memory-toast-${Date.now()}-${Math.random().toString(36).slice(2)}`,
			action: 'compact',
			label: LABELS.compact,
			preview
		};
		toasts = [...toasts, toast].slice(-MAX_TOASTS);
		timers.set(
			toast.id,
			setTimeout(() => dismiss(toast.id), TOAST_DURATION_MS)
		);
	}

	return {
		get toasts() {
			return toasts;
		},
		add,
		addCompaction,
		dismiss
	};
}

export const memoryToastStore = createMemoryToastStore();
