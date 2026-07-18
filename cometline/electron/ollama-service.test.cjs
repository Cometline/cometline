'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const {
	createOllamaService,
	isValidModelName,
	normalizeNativeBase,
	resolvePullName
} = require('./ollama-service.cjs');

test('normalizeNativeBase strips /v1', () => {
	assert.equal(normalizeNativeBase('http://127.0.0.1:11434/v1/'), 'http://127.0.0.1:11434');
});

test('isValidModelName rejects URLs', () => {
	assert.equal(isValidModelName('llama3.2:3b'), true);
	assert.equal(isValidModelName('http://evil'), false);
});

test('resolvePullName maps catalog ids', () => {
	assert.equal(resolvePullName({ catalogId: 'private-memory' }), 'qwen3-embedding:0.6b');
	assert.throws(() => resolvePullName({ catalogId: 'nope' }));
});

test('checkHealth rejects non-loopback bases', async () => {
	const service = createOllamaService({
		fetchImpl: async () => {
			throw new Error('should not fetch');
		}
	});
	await assert.rejects(() => service.checkHealth('https://example.com'), /loopback/i);
});

test('pullModel streams progress and refreshes tags', async () => {
	const progress = [];
	const service = createOllamaService({
		sendProgress: (payload) => progress.push(payload),
		fetchImpl: async (url, init = {}) => {
			const href = String(url);
			if (href.endsWith('/api/version')) {
				return {
					ok: true,
					json: async () => ({ version: '0.9.0' })
				};
			}
			if (href.endsWith('/api/pull')) {
				const encoder = new TextEncoder();
				const chunks = [
					JSON.stringify({ status: 'downloading', completed: 50, total: 100 }) + '\n',
					JSON.stringify({ status: 'success' }) + '\n'
				];
				let i = 0;
				return {
					ok: true,
					body: {
						getReader() {
							return {
								async read() {
									if (i >= chunks.length) return { done: true, value: undefined };
									const value = encoder.encode(chunks[i++]);
									return { done: false, value };
								}
							};
						}
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
		catalogId: 'private-memory'
	});
	assert.equal(result.ok, true);
	assert.equal(result.model, 'qwen3-embedding:0.6b');
	assert.equal(result.models[0].name, 'qwen3-embedding:0.6b');
	assert.ok(progress.some((p) => p.percent === 50));
});
