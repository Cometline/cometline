import { getReasoningSegments } from '$lib/conversation/reasoning';
import type { ChatItem } from '$lib/types';

export const CONTEXT_WINDOW_LIMIT_OPTIONS = [128_000, 256_000] as const;
export type ContextWindowLimit = (typeof CONTEXT_WINDOW_LIMIT_OPTIONS)[number];
export const DEFAULT_CONTEXT_WINDOW_LIMIT: ContextWindowLimit = 128_000;

export function normalizeContextWindowLimit(value: unknown): ContextWindowLimit {
	return Number(value) === 256_000 ? 256_000 : 128_000;
}

export function resolveContextWindow(limit?: ContextWindowLimit | number | null): number {
	return normalizeContextWindowLimit(limit ?? DEFAULT_CONTEXT_WINDOW_LIMIT);
}

export function formatContextWindow(tokens: number): string {
	if (!Number.isFinite(tokens) || tokens <= 0) return '';
	if (tokens >= 1_000_000) {
		const millions = tokens / 1_000_000;
		return millions % 1 === 0 ? `${millions}M` : `${millions.toFixed(1)}M`;
	}
	if (tokens >= 1_000) {
		const thousands = tokens / 1_000;
		return thousands % 1 === 0 ? `${thousands}k` : `${thousands.toFixed(1)}k`;
	}
	return String(Math.round(tokens));
}

/** Mirrors CometMind chars/4 token estimate for UI usage display. */
export function estimateTokensFromText(text: string): number {
	const n = [...text].length;
	if (n <= 0) return 0;
	const tokens = Math.floor(n / 4);
	return tokens < 1 && n > 0 ? 1 : tokens;
}

/** Estimates prompt tokens from visible transcript items (approximate). */
export function estimateChatContextTokens(items: ChatItem[]): number {
	let total = 0;
	for (const item of items) {
		switch (item.type) {
			case 'user':
				total += estimateTokensFromText(item.text);
				break;
			case 'assistant':
				total += estimateTokensFromText(item.text);
				for (const segment of getReasoningSegments(item.reasoning)) {
					total += estimateTokensFromText(segment.text);
				}
				break;
			case 'tool':
				total += estimateTokensFromText(item.toolName);
				total += estimateTokensFromText(JSON.stringify(item.input));
				break;
			case 'status':
				total += estimateTokensFromText(item.text);
				break;
			case 'memory':
				for (const memory of item.memories) {
					total += estimateTokensFromText(memory.content);
				}
				break;
			case 'error':
				total += estimateTokensFromText(item.text);
				break;
			case 'subagent':
				total += estimateTokensFromText(item.purpose);
				total += estimateTokensFromText(item.agentName);
				total += estimateTokensFromText(item.summary ?? '');
				for (const entry of item.progress) {
					if (entry.kind === 'stream') {
						total += estimateTokensFromText(entry.text);
					} else if (entry.kind === 'tool') {
						total += estimateTokensFromText(entry.title);
					} else {
						total += estimateTokensFromText(entry.text);
					}
				}
				break;
		}
	}
	return total;
}

export function formatContextUsageTokens(tokens: number): string {
	if (!Number.isFinite(tokens) || tokens <= 0) return '0';
	if (tokens >= 1_000_000) {
		const millions = tokens / 1_000_000;
		return millions % 1 === 0 ? `${millions}M` : `${millions.toFixed(1)}M`;
	}
	if (tokens >= 1_000) {
		const thousands = tokens / 1_000;
		return thousands % 1 === 0 ? `${thousands}K` : `${thousands.toFixed(1)}K`;
	}
	return String(Math.round(tokens));
}

export function formatContextPercent(used: number, limit: number): string {
	if (!Number.isFinite(limit) || limit <= 0) return '0';
	const percent = Math.min(100, (used / limit) * 100);
	return percent % 1 === 0 ? String(percent) : percent.toFixed(1);
}
