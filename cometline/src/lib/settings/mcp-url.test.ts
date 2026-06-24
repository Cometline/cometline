import { describe, expect, it } from 'vitest';
import type { MCPServerConfig } from '$lib/cometmind-settings';
import {
	appendQueryParam,
	looksLikeApiKeyHeader,
	normalizeHttpConnection,
	normalizeServerConnection
} from './mcp-url';

describe('looksLikeApiKeyHeader', () => {
	it('matches common API key / token header names', () => {
		for (const name of [
			'API_KEY',
			'api-key',
			'TYPEFULLY_API_KEY',
			'CONTEXT7_API_KEY',
			'X_API_TOKEN',
			'accessToken',
			'token',
			'KEY'
		]) {
			expect(looksLikeApiKeyHeader(name), name).toBe(true);
		}
	});

	it('does not match Authorization or arbitrary headers', () => {
		for (const name of ['Authorization', 'Content-Type', 'X-Custom', 'user-agent', '']) {
			expect(looksLikeApiKeyHeader(name), name).toBe(false);
		}
	});
});

describe('appendQueryParam', () => {
	it('adds the first query parameter with ?', () => {
		expect(appendQueryParam('https://x.com/mcp', 'KEY', 'abc')).toBe(
			'https://x.com/mcp?KEY=abc'
		);
	});

	it('appends to an existing query string with &', () => {
		expect(appendQueryParam('https://x.com/mcp?a=1', 'KEY', 'abc')).toBe(
			'https://x.com/mcp?a=1&KEY=abc'
		);
	});

	it('preserves a fragment', () => {
		expect(appendQueryParam('https://x.com/mcp#frag', 'KEY', 'abc')).toBe(
			'https://x.com/mcp?KEY=abc#frag'
		);
	});

	it('does not duplicate an existing parameter (case-insensitive)', () => {
		expect(appendQueryParam('https://x.com/mcp?key=old', 'KEY', 'new')).toBe(
			'https://x.com/mcp?key=old'
		);
	});

	it('encodes name and value', () => {
		expect(appendQueryParam('https://x.com/mcp', 'A B', 'a/b')).toBe(
			'https://x.com/mcp?A%20B=a%2Fb'
		);
	});
});

describe('normalizeHttpConnection', () => {
	it('folds an API-key header into the URL when there is no query string (Typefully case)', () => {
		const result = normalizeHttpConnection('https://mcp.typefully.com/mcp', {
			TYPEFULLY_API_KEY: 'secret'
		});
		expect(result.url).toBe('https://mcp.typefully.com/mcp?TYPEFULLY_API_KEY=secret');
		expect(result.headers).toEqual({});
		expect(result.movedToQuery).toEqual(['TYPEFULLY_API_KEY']);
	});

	it('folds a header whose name matches an existing query parameter', () => {
		const result = normalizeHttpConnection('https://x.com/mcp?TYPEFULLY_API_KEY=', {
			TYPEFULLY_API_KEY: 'secret'
		});
		// existing param wins; the redundant custom header is dropped
		expect(result.url).toBe('https://x.com/mcp?TYPEFULLY_API_KEY=');
		expect(result.headers).toEqual({});
		expect(result.movedToQuery).toEqual(['TYPEFULLY_API_KEY']);
	});

	it('leaves Authorization Bearer headers untouched', () => {
		const result = normalizeHttpConnection('https://x.com/mcp', {
			Authorization: 'Bearer secret'
		});
		expect(result.url).toBe('https://x.com/mcp');
		expect(result.headers).toEqual({ Authorization: 'Bearer secret' });
		expect(result.movedToQuery).toEqual([]);
	});

	it('does not fold when the URL already has an unrelated query string', () => {
		const result = normalizeHttpConnection('https://x.com/mcp?org=acme', {
			API_KEY: 'secret'
		});
		expect(result.url).toBe('https://x.com/mcp?org=acme');
		expect(result.headers).toEqual({ API_KEY: 'secret' });
		expect(result.movedToQuery).toEqual([]);
	});

	it('keeps non-credential headers', () => {
		const result = normalizeHttpConnection('https://x.com/mcp', {
			'X-Custom': 'v',
			API_KEY: 'secret'
		});
		expect(result.url).toBe('https://x.com/mcp?API_KEY=secret');
		expect(result.headers).toEqual({ 'X-Custom': 'v' });
		expect(result.movedToQuery).toEqual(['API_KEY']);
	});

	it('preserves a pasted full URL with the key already in the query string', () => {
		const result = normalizeHttpConnection(
			'https://mcp.typefully.com/mcp?TYPEFULLY_API_KEY=k',
			{}
		);
		expect(result.url).toBe('https://mcp.typefully.com/mcp?TYPEFULLY_API_KEY=k');
		expect(result.movedToQuery).toEqual([]);
	});
});

describe('normalizeServerConnection', () => {
	const base: MCPServerConfig = {
		id: 'server-1',
		name: 'Typefully',
		enabled: true,
		transport: 'http',
		command: '',
		args: [],
		env: {},
		url: 'https://mcp.typefully.com/mcp',
		headers: { TYPEFULLY_API_KEY: 'secret' }
	};

	it('rewrites an http server with a misplaced key header', () => {
		const result = normalizeServerConnection(base);
		expect(result).not.toBe(base);
		expect(result.url).toBe('https://mcp.typefully.com/mcp?TYPEFULLY_API_KEY=secret');
		expect(result.headers).toEqual({});
	});

	it('returns the same reference for stdio servers', () => {
		const stdio: MCPServerConfig = {
			...base,
			transport: 'stdio',
			url: '',
			headers: {},
			command: 'npx'
		};
		expect(normalizeServerConnection(stdio)).toBe(stdio);
	});

	it('returns the same reference when nothing needs changing', () => {
		const clean: MCPServerConfig = {
			...base,
			url: 'https://mcp.typefully.com/mcp?TYPEFULLY_API_KEY=secret',
			headers: {}
		};
		expect(normalizeServerConnection(clean)).toBe(clean);
	});
});
