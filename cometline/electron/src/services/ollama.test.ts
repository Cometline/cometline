import { describe, expect, it } from 'vitest';
import {
	createOllamaService,
	isValidModelName,
	normalizeNativeBase,
	resolvePullName
} from './ollama.js';

describe('Ollama service', () => {
	it('normalizes native bases that end in /v1', () => {
		expect(normalizeNativeBase('http://127.0.0.1:11434/v1/')).toBe('http://127.0.0.1:11434');
	});

	it('rejects URLs as model names', () => {
		expect(isValidModelName('llama3.2:3b')).toBe(true);
		expect(isValidModelName('http://evil')).toBe(false);
	});

	it('maps catalog identifiers to pull names', () => {
		expect(resolvePullName({ catalogId: 'private-memory', modelName: undefined })).toBe(
			'qwen3-embedding:0.6b'
		);
		expect(() => resolvePullName({ catalogId: 'nope', modelName: undefined })).toThrow();
	});

	it('rejects non-loopback health checks', async () => {
		const service = createOllamaService({
			fetchImpl: async () => {
				throw new Error('should not fetch');
			}
		});
		await expect(service.checkHealth('https://example.com')).rejects.toThrow(/loopback/i);
	});

	it('streams pull progress and refreshes model tags', async () => {
		const progress: Array<{ percent?: number }> = [];
		const service = createOllamaService({
			sendProgress: (payload: { percent?: number }) => progress.push(payload),
			fetchImpl: async (url: string, init: RequestInit = {}) => {
				const href = String(url);
				if (href.endsWith('/api/version'))
					return { ok: true, json: async () => ({ version: '0.9.0' }) };
				if (href.endsWith('/api/pull')) {
					const encoder = new TextEncoder();
					const chunks = [
						`${JSON.stringify({ status: 'downloading', completed: 50, total: 100 })}\n`,
						`${JSON.stringify({ status: 'success' })}\n`
					];
					let index = 0;
					return {
						ok: true,
						body: {
							getReader: () => ({
								read: async () =>
									index >= chunks.length
										? { done: true, value: undefined }
										: { done: false, value: encoder.encode(chunks[index++]) }
							})
						}
					};
				}
				if (href.endsWith('/api/tags')) {
					return {
						ok: true,
						json: async () => ({
							models: [{ name: 'qwen3-embedding:0.6b', size: 100 }]
						})
					};
				}
				throw new Error(`unexpected url ${href} ${JSON.stringify(init)}`);
			}
		});

		const result = await service.pullModel({
			baseURL: 'http://127.0.0.1:11434',
			catalogId: 'private-memory',
			modelName: undefined
		});
		expect(result.ok).toBe(true);
		expect(result.model).toBe('qwen3-embedding:0.6b');
		expect(result.models[0]?.name).toBe('qwen3-embedding:0.6b');
		expect(progress.some((item) => item.percent === 50)).toBe(true);
	});
});
