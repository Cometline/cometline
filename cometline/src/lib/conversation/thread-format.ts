import { assistantThinkingWaitStatus } from './assistant-wait-status';
import { formatToolDisplayName, type McpServerNameLookup } from './mcp-tool-display';
import type { ChatItem } from '$lib/stores/chat.svelte';

export function formatToolDuration(ms: number) {
	if (ms < 1000) return `${Math.max(1, Math.round(ms))}ms`;
	if (ms < 10000) return `${(ms / 1000).toFixed(1)}s`;
	return `${Math.round(ms / 1000)}s`;
}

export function toolDurationLabel(item: Extract<ChatItem, { type: 'tool' }>, now: number) {
	if (item.durationMs != null) return formatToolDuration(item.durationMs);
	if (item.pending && item.startedAt != null) return formatToolDuration(now - item.startedAt);
	return '';
}

export function toolFoldLabel(
	item: Extract<ChatItem, { type: 'tool' }>,
	now: number,
	mcpServers: McpServerNameLookup[] = []
) {
	const displayName = formatToolDisplayName(item.toolName, mcpServers);
	const status = item.pending ? 'running' : item.error ? 'fail' : 'success';
	const duration = toolDurationLabel(item, now);
	return duration
		? `${displayName} → ${status} · ${duration}`
		: `${displayName} → ${status}`;
}

export function usageText(item: Extract<ChatItem, { type: 'status' }>) {
	const usage = item.usage;
	if (!usage) return item.text;
	return `${item.text} · ${usage.input_tokens} in / ${usage.output_tokens} out`;
}

export function assistantWaitSeconds(
	item: Extract<ChatItem, { type: 'assistant' }> | undefined,
	now: number
) {
	if (!item || item.pendingStartedAt == null) return 0;
	return Math.max(0, Math.floor((now - item.pendingStartedAt) / 1000));
}

export function assistantThinkingWait(
	item: Extract<ChatItem, { type: 'assistant' }> | undefined,
	now: number
) {
	return assistantThinkingWaitStatus(
		item?.activityPhase,
		item?.activityMessage,
		assistantWaitSeconds(item, now)
	);
}
