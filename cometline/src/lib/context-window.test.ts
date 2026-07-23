import { describe, expect, it } from 'vitest';
import {
	DEFAULT_CONTEXT_WINDOW_LIMIT,
	estimateChatContextTokens,
	estimateTokensFromText,
	formatContextPercent,
	formatContextUsageTokens,
	formatContextWindow,
	normalizeContextWindowLimit,
	resolveContextWindow,
	resolveContextWindowUsage
} from './context-window';

describe('context-window', () => {
	it('normalizes to 128k or 256k only', () => {
		expect(normalizeContextWindowLimit(256_000)).toBe(256_000);
		expect(normalizeContextWindowLimit(128_000)).toBe(128_000);
		expect(normalizeContextWindowLimit(200_000)).toBe(128_000);
		expect(normalizeContextWindowLimit(undefined)).toBe(128_000);
	});

	it('resolves configured limit', () => {
		expect(resolveContextWindow()).toBe(DEFAULT_CONTEXT_WINDOW_LIMIT);
		expect(resolveContextWindow(256_000)).toBe(256_000);
	});

	it('formats large windows compactly', () => {
		expect(formatContextWindow(128_000)).toBe('128k');
		expect(formatContextWindow(256_000)).toBe('256k');
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
			budget: { estimated: 1000, available: 125_952, contextWindow: 128_000 },
			items: [{ id: '1', type: 'user', text: 'ignored when server budget present' }],
			draftText: 'abcd',
			contextWindowLimit: 128_000,
			maxTokens: 2048
		});
		expect(usage.source).toBe('server');
		expect(usage.used).toBe(1001);
		expect(usage.limit).toBe(125_952);
	});

	it('falls back to transcript estimate with available denominator', () => {
		const usage = resolveContextWindowUsage({
			budget: null,
			items: [{ id: '1', type: 'user', text: 'abcd' }],
			draftText: '',
			contextWindowLimit: 128_000,
			maxTokens: 2048
		});
		expect(usage.source).toBe('fallback');
		expect(usage.used).toBe(1);
		expect(usage.limit).toBe(128_000 - 2048);
	});
});
