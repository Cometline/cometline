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
	resolveContextAvailableBudget,
	resolveContextWindow,
	resolveContextWindowUsage
} from './context-window';

describe('context-window', () => {
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

	it('caps output at min(model output, 32k)', () => {
		expect(effectiveMaxTokens(8_192)).toBe(8_192);
		expect(effectiveMaxTokens(128_000)).toBe(32_000);
		expect(effectiveMaxTokens(null)).toBe(32_000);
	});

	it('uses max(effective, 20k) reserve for available budget', () => {
		expect(resolveContextAvailableBudget(128_000, 8_192)).toBe(
			128_000 - COMPACTION_OUTPUT_BUFFER
		);
		expect(resolveContextAvailableBudget(200_000, 64_000)).toBe(200_000 - 32_000);
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
			contextWindow: 128_000
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
			contextWindow: 200_000,
			modelOutput: 64_000
		});
		expect(usage.source).toBe('fallback');
		expect(usage.used).toBe(1);
		expect(usage.limit).toBe(200_000 - 32_000);
	});

	it('falls back to the default window when no model context is known', () => {
		const usage = resolveContextWindowUsage({
			budget: null,
			items: [],
			draftText: ''
		});
		expect(usage.limit).toBe(DEFAULT_CONTEXT_WINDOW_LIMIT - 32_000);
	});
});
