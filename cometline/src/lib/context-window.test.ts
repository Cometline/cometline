import { describe, expect, it } from 'vitest';
import {
	COMPACTION_OUTPUT_BUFFER,
	DEFAULT_CONTEXT_WINDOW_LIMIT,
	effectiveMaxTokens,
	estimateChatContextTokens,
	estimateTokensFromText,
	formatContextPercent,
	formatContextUsageTokens,
	formatContextWindow,
	normalizeContextWindowLimit,
	resolveContextAvailableBudget,
	resolveContextWindow,
	resolveContextWindowUsage
} from './context-window';

describe('context-window', () => {
	it('normalizes legacy 128k/256k settings values', () => {
		expect(normalizeContextWindowLimit(256_000)).toBe(256_000);
		expect(normalizeContextWindowLimit(128_000)).toBe(128_000);
		expect(normalizeContextWindowLimit(200_000)).toBe(128_000);
		expect(normalizeContextWindowLimit(undefined)).toBe(128_000);
	});

	it('resolves positive context windows including per-model values', () => {
		expect(resolveContextWindow()).toBe(DEFAULT_CONTEXT_WINDOW_LIMIT);
		expect(resolveContextWindow(256_000)).toBe(256_000);
		expect(resolveContextWindow(200_000)).toBe(200_000);
	});

	it('formats large windows compactly', () => {
		expect(formatContextWindow(128_000)).toBe('128k');
		expect(formatContextWindow(256_000)).toBe('256k');
		expect(formatContextWindow(1_000_000)).toBe('1M');
	});

	it('caps effective max tokens by catalog output', () => {
		expect(effectiveMaxTokens(8192, 4096)).toBe(4096);
		expect(effectiveMaxTokens(2048, 128_000)).toBe(2048);
		expect(effectiveMaxTokens(4096, 0)).toBe(4096);
		expect(effectiveMaxTokens(null, null)).toBe(2048);
	});

	it('uses max(effective, 20k) reserve for available budget', () => {
		expect(resolveContextAvailableBudget(128_000, 2048)).toBe(128_000 - COMPACTION_OUTPUT_BUFFER);
		expect(resolveContextAvailableBudget(200_000, 64_000, 32_000)).toBe(200_000 - 32_000);
	});

	it('estimates tokens from text with chars/4 heuristic', () => {
		expect(estimateTokensFromText('')).toBe(0);
		expect(estimateTokensFromText('abcd')).toBe(1);
		expect(estimateTokensFromText('a'.repeat(400))).toBe(100);
	});

	it('estimates transcript tokens from chat items', () => {
		const items = [
			{ id: '1', type: 'user' as const, text: 'hello world' },
			{ id: '2', type: 'assistant' as const, text: 'hi there' }
		];
		expect(estimateChatContextTokens(items)).toBeGreaterThan(0);
	});

	it('includes prompt-sized tool output in transcript estimates', () => {
		const base = estimateChatContextTokens([
			{ id: 't1', type: 'tool' as const, toolName: 'read_file', input: {}, output: '' }
		]);
		const withOutput = estimateChatContextTokens([
			{
				id: 't1',
				type: 'tool' as const,
				toolName: 'read_file',
				input: {},
				output: 'x'.repeat(8000)
			}
		]);

		expect(withOutput).toBeGreaterThan(base);
		expect(withOutput - base).toBeLessThan(1200);
	});

	it('formats usage tooltip values', () => {
		expect(formatContextUsageTokens(180_400)).toBe('180.4K');
		expect(formatContextPercent(180_400, 256_000)).toBe('70.5');
	});

	it('prefers server budget and adds draft tokens', () => {
		const usage = resolveContextWindowUsage({
			budget: { estimated: 1000, available: 108_000, contextWindow: 128_000 },
			items: [{ id: '1', type: 'user', text: 'ignored when server budget present' }],
			draftText: 'abcd',
			contextWindowLimit: 128_000,
			maxTokens: 2048
		});
		expect(usage.source).toBe('server');
		expect(usage.used).toBe(1001);
		expect(usage.limit).toBe(108_000);
	});

	it('falls back to transcript estimate with model-aware denominator', () => {
		const usage = resolveContextWindowUsage({
			budget: null,
			items: [{ id: '1', type: 'user', text: 'abcd' }],
			draftText: '',
			contextWindowLimit: 200_000,
			maxTokens: 8192,
			modelOutput: 32_000
		});
		expect(usage.source).toBe('fallback');
		expect(usage.used).toBe(1);
		expect(usage.limit).toBe(200_000 - COMPACTION_OUTPUT_BUFFER);
	});

	it('does not expose a required contextWindowLimit UI path for fallback', () => {
		const usage = resolveContextWindowUsage({
			budget: null,
			items: [],
			draftText: '',
			maxTokens: 2048
		});
		expect(usage.limit).toBe(DEFAULT_CONTEXT_WINDOW_LIMIT - COMPACTION_OUTPUT_BUFFER);
	});
});
