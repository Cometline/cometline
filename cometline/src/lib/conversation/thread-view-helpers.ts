import type { ChatItem } from '$lib/stores/chat.svelte';
import type { TimelineEntry } from './thinking-attribution';

export function speakerFor(item: ChatItem | undefined): 'user' | 'assistant' | null {
	if (!item) return null;
	if (item.type === 'user') return 'user';
	if (item.type === 'assistant' || item.type === 'tool' || item.type === 'subagent')
		return 'assistant';
	return null;
}

export function startsSpeakerRun(
	items: readonly ChatItem[],
	index: number,
	speaker: 'user' | 'assistant'
) {
	for (let i = index - 1; i >= 0; i--) {
		const previousSpeaker = speakerFor(items[i]);
		if (!previousSpeaker) continue;
		return previousSpeaker !== speaker;
	}
	return true;
}

export function timelineEntryKey(entry: TimelineEntry) {
	if (entry.kind === 'reasoning') return `${entry.kind}-${entry.segmentIndex}`;
	if (entry.kind === 'memory') return entry.kind;
	if (entry.kind === 'tool') return `${entry.kind}-${entry.tool.id}`;
	if (entry.kind === 'subagent') return `${entry.kind}-${entry.subagent.id}`;
	return `${entry.kind}-${entry.error.id}`;
}
