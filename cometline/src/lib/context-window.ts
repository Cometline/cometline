import { getReasoningSegments } from '$lib/conversation/reasoning';
import type { ChatItem } from '$lib/types';

/** @deprecated Global 128k/256k UI removed; kept for settings JSON backward compat. */
export const CONTEXT_WINDOW_LIMIT_OPTIONS = [128_000, 256_000] as const;
/** @deprecated */
export type ContextWindowLimit = (typeof CONTEXT_WINDOW_LIMIT_OPTIONS)[number];
export const DEFAULT_CONTEXT_WINDOW_LIMIT = 128_000;
/** Matches CometMind CompactionOutputBuffer. */
export const COMPACTION_OUTPUT_BUFFER = 20_000;
const TOOL_RESULT_PROMPT_RUNE_LIMIT = 4000;

/** @deprecated Prefer the model catalog's per-model context value. */
export function normalizeContextWindowLimit(value: unknown): ContextWindowLimit {
	return Number(value) === 256_000 ? 256_000 : 128_000;
}

/** @deprecated Prefer model.context / ResolveSessionBudget. */
export function resolveContextWindow(limit?: ContextWindowLimit | number | null): number {
	const n = Number(limit);
	if (Number.isFinite(n) && n > 0) return Math.floor(n);
	return DEFAULT_CONTEXT_WINDOW_LIMIT;
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

function promptSizedToolOutput(text: string): string {
	const runes = [...text];
	if (runes.length <= TOOL_RESULT_PROMPT_RUNE_LIMIT) return text;
	return `${runes.slice(0, TOOL_RESULT_PROMPT_RUNE_LIMIT).join('')}\n\n(tool result truncated for prompt)`;
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
				if (item.output) total += estimateTokensFromText(promptSizedToolOutput(item.output));
				if (item.error) total += estimateTokensFromText(promptSizedToolOutput(item.error));
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

export type ContextBudgetSnapshot = {
	estimated: number;
	available: number;
	contextWindow: number;
	compacted?: boolean;
};

export type ContextWindowUsage = {
	used: number;
	limit: number;
	source: 'server' | 'fallback';
};

export type ModelLimitHints = {
	context?: number | null;
	output?: number | null;
};

/** effectiveMaxTokens = min(userMaxTokens, catalogOutput) when catalogOutput > 0 */
export function effectiveMaxTokens(userMaxTokens?: number | null, catalogOutput?: number | null): number {
	const user = Number.isFinite(userMaxTokens) && (userMaxTokens as number) > 0 ? Math.floor(userMaxTokens as number) : 2048;
	const output = Number.isFinite(catalogOutput) && (catalogOutput as number) > 0 ? Math.floor(catalogOutput as number) : 0;
	if (output > 0 && output < user) return output;
	return user;
}

/**
 * Available prompt budget matching ResolveSessionBudget:
 * reserve = max(effectiveMaxTokens, 20_000); available = context - reserve.
 */
export function resolveContextAvailableBudget(
	contextWindow?: number | null,
	maxTokens?: number | null,
	catalogOutput?: number | null
): number {
	const window = resolveContextWindow(contextWindow ?? DEFAULT_CONTEXT_WINDOW_LIMIT);
	const effective = effectiveMaxTokens(maxTokens, catalogOutput);
	const reserve = Math.max(effective, COMPACTION_OUTPUT_BUFFER);
	return Math.max(1, window - reserve);
}

/**
 * Prefer server context_budget (same math as MaybeCompact); fall back to visible
 * transcript estimate with the same available-budget denominator.
 */
export function resolveContextWindowUsage(input: {
	budget: ContextBudgetSnapshot | null | undefined;
	items: ChatItem[];
	draftText: string;
	contextWindowLimit?: number | null;
	maxTokens?: number | null;
	modelOutput?: number | null;
}): ContextWindowUsage {
	const draftTokens = input.draftText.trim() ? estimateTokensFromText(input.draftText) : 0;
	if (input.budget && Number.isFinite(input.budget.available) && input.budget.available > 0) {
		return {
			used: Math.max(0, input.budget.estimated) + draftTokens,
			limit: Math.max(1, input.budget.available),
			source: 'server'
		};
	}
	return {
		used: estimateChatContextTokens(input.items) + draftTokens,
		limit: resolveContextAvailableBudget(input.contextWindowLimit, input.maxTokens, input.modelOutput),
		source: 'fallback'
	};
}
