import type { ChatItem } from '$lib/stores/chat.svelte';

export interface ThreadClocksDeps {
	getThreadItems: () => readonly ChatItem[];
	getSessionStreaming: () => boolean;
	getStreamingAssistantId: () => string | null;
	hasStandaloneMemoryEvents: () => boolean;
}

export function createThreadClocks(deps: ThreadClocksDeps) {
	let copiedId = $state<string | null>(null);
	let copyResetTimer: ReturnType<typeof setTimeout> | null = null;
	let memoryCycleTick = $state(0);
	let now = $state(Date.now());

	async function copyMessage(id: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			return;
		}
		copiedId = id;
		if (copyResetTimer) clearTimeout(copyResetTimer);
		copyResetTimer = setTimeout(() => {
			copiedId = null;
			copyResetTimer = null;
		}, 1600);
	}

	$effect(() => {
		return () => {
			if (copyResetTimer) clearTimeout(copyResetTimer);
		};
	});

	$effect(() => {
		if (!deps.hasStandaloneMemoryEvents()) {
			memoryCycleTick = 0;
			return;
		}
		const timer = setInterval(() => memoryCycleTick++, 5000);
		return () => clearInterval(timer);
	});

	$effect(() => {
		const items = deps.getThreadItems();
		const sessionStreaming = deps.getSessionStreaming();
		const streamingAssistantId = deps.getStreamingAssistantId();
		const hasTimedPending = items.some(
			(item) =>
				(item.type === 'tool' && item.pending) ||
				(item.type === 'assistant' &&
					item.pendingStartedAt != null &&
					sessionStreaming &&
					item.id === streamingAssistantId &&
					!item.text?.trim())
		);
		if (!hasTimedPending) return;
		const timer = setInterval(() => {
			now = Date.now();
		}, 100);
		return () => clearInterval(timer);
	});

	return {
		get copiedId() {
			return copiedId;
		},
		get memoryCycleTick() {
			return memoryCycleTick;
		},
		get now() {
			return now;
		},
		copyMessage
	};
}
