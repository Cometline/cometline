import { describe, expect, it } from 'vitest';
import {
	featuredOllamaCatalog,
	getOllamaCatalogEntry,
	isValidOllamaModelName,
	OLLAMA_CATALOG
} from './catalog';
import { isLoopbackOllamaURL, normalizeOllamaNativeBase, ollamaChatBaseURL } from './url';

describe('ollama catalog', () => {
	it('ships three featured cards including Private Memory', () => {
		const featured = featuredOllamaCatalog();
		expect(featured).toHaveLength(3);
		expect(getOllamaCatalogEntry('private-memory')?.pullName).toBe('qwen3-embedding:0.6b');
		expect(OLLAMA_CATALOG.every((e) => !e.capabilities.agent && !e.capabilities.extraction)).toBe(
			true
		);
	});

	it('validates model names for advanced pull', () => {
		expect(isValidOllamaModelName('llama3.2:3b')).toBe(true);
		expect(isValidOllamaModelName('qwen3-embedding:0.6b')).toBe(true);
		expect(isValidOllamaModelName('http://evil')).toBe(false);
		expect(isValidOllamaModelName('../etc/passwd')).toBe(false);
		expect(isValidOllamaModelName('')).toBe(false);
	});
});

describe('ollama url', () => {
	it('stores native base and derives /v1 for chat', () => {
		expect(normalizeOllamaNativeBase('http://127.0.0.1:11434/v1')).toBe(
			'http://127.0.0.1:11434'
		);
		expect(ollamaChatBaseURL('http://127.0.0.1:11434')).toBe('http://127.0.0.1:11434/v1');
		expect(isLoopbackOllamaURL('http://127.0.0.1:11434')).toBe(true);
		expect(isLoopbackOllamaURL('https://api.example.com')).toBe(false);
	});
});
