import crypto from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { createProviderAuth } from './provider-auth.js';

const temporaryDirectories: string[] = [];

afterEach(() => {
	for (const directory of temporaryDirectories.splice(0)) {
		fs.rmSync(directory, { force: true, recursive: true });
	}
});

function createDomain(homeDirectory: string, fetchMock: typeof fetch) {
	return createProviderAuth({
		fs,
		path,
		http,
		crypto,
		fetch: fetchMock,
		platform: { homedir: () => homeDirectory, environment: {} },
		window: {
			openExternal: vi.fn(async () => undefined),
			showMessageBox: vi.fn(async () => undefined)
		},
		ollama: { listModels: vi.fn(async () => ({ models: [] })) }
	});
}

describe('provider auth domain', () => {
	it('normalizes OpenAI-compatible model endpoints and returns sorted unique models', async () => {
		const request = vi.fn(async () => new Response(JSON.stringify({ data: [{ id: 'z' }, { id: 'a' }, { id: 'z' }] })));
		const domain = createDomain('/unused', request as unknown as typeof fetch);
		await expect(
			domain.fetchProviderModels({ method: 'openai', baseURL: ' https://models.example.test/v1/ ', apiKey: 'key' })
		).resolves.toEqual({ models: ['a', 'z'] });
		expect(request).toHaveBeenCalledWith(
			'https://models.example.test/v1/models',
			expect.objectContaining({ headers: { Authorization: 'Bearer key', Accept: 'application/json' } })
		);
	});

	it('uses the injected platform home for Codex status and Cursor MCP import', () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-provider-auth-'));
		temporaryDirectories.push(home);
		const domain = createDomain(home, vi.fn() as unknown as typeof fetch);
		const codexDirectory = path.join(home, '.codex');
		fs.mkdirSync(codexDirectory, { recursive: true });
		fs.writeFileSync(path.join(codexDirectory, 'auth.json'), JSON.stringify({
			auth_mode: 'chatgpt',
			tokens: { access_token: 'token', account_id: 'account' }
		}));
		const cursorDirectory = path.join(home, '.cursor');
		fs.mkdirSync(cursorDirectory, { recursive: true });
		fs.writeFileSync(path.join(cursorDirectory, 'mcp.json'), '{"mcpServers":{"docs":{}}}');
		expect(domain.getCodexAuthStatus()).toMatchObject({ authenticated: true, accountID: 'account' });
		expect(domain.readCursorMcpConfig()).toEqual({
			ok: true,
			path: path.join(cursorDirectory, 'mcp.json'),
			config: { mcpServers: { docs: {} } }
		});
	});

	it('refreshes an expired xAI subscription session under its auth-file lock', async () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-provider-auth-'));
		temporaryDirectories.push(home);
		const authDirectory = path.join(home, '.cometmind', 'xai');
		fs.mkdirSync(authDirectory, { recursive: true });
		const authPath = path.join(authDirectory, 'auth.json');
		fs.writeFileSync(authPath, JSON.stringify({
			auth_mode: 'subscription',
			tokens: { access_token: 'expired', refresh_token: 'refresh', expires_at: 1 }
		}));
		const request = vi
			.fn()
			.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'fresh', expires_in: 3600 })))
			.mockResolvedValueOnce(new Response(JSON.stringify({ data: [{ id: 'grok-4' }] })));
		const domain = createDomain(home, request as unknown as typeof fetch);
		await expect(domain.fetchProviderModels({ method: 'xai', baseURL: '', apiKey: '' })).resolves.toEqual({ models: ['grok-4'] });
		expect(JSON.parse(fs.readFileSync(authPath, 'utf8')).tokens.access_token).toBe('fresh');
		expect(fs.existsSync(`${authPath}.lock`)).toBe(false);
	});

	it('synthesizes grok-4.6-fast next to grok-4.6 for xAI subscription catalogs', async () => {
		const home = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-provider-auth-'));
		temporaryDirectories.push(home);
		const authDirectory = path.join(home, '.cometmind', 'xai');
		fs.mkdirSync(authDirectory, { recursive: true });
		fs.writeFileSync(path.join(authDirectory, 'auth.json'), JSON.stringify({
			auth_mode: 'subscription',
			tokens: { access_token: 'fresh', refresh_token: 'refresh', expires_at: Date.now() + 10 * 60 * 1000 }
		}));
		const request = vi.fn(async () => new Response(JSON.stringify({ data: [{ id: 'grok-4.6' }, { id: 'grok-4.5' }] })));
		const domain = createDomain(home, request as unknown as typeof fetch);
		await expect(domain.fetchProviderModels({ method: 'xai', baseURL: '', apiKey: '' })).resolves.toEqual({
			models: ['grok-4.5', 'grok-4.6', 'grok-4.6-fast']
		});
	});
});
