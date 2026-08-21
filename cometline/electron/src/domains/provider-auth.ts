import type { ProviderConfig } from '../../../src/lib/types.js';

const FETCH_MODELS_TIMEOUT_MS = 30_000;
const CODEX_BASE_URL = 'https://chatgpt.com/backend-api/codex';
const CODEX_CLIENT_ID = 'app_EMoamEEZ73f0CkXaXp7hrann';
const CODEX_REFRESH_URL = 'https://auth.openai.com/oauth/token';
const CODEX_CLIENT_VERSION = '1.0.0';
const CODEX_AUTH_CALLBACK_PORT = 1455;
const CODEX_AUTH_CALLBACK_PATH = '/auth/callback';
const CODEX_AUTH_TIMEOUT_MS = 5 * 60 * 1000;
const XAI_BASE_URL = 'https://api.x.ai/v1';
const XAI_CLIENT_ID = 'b1a00492-073a-47ea-816f-4c329264a828';
const XAI_TOKEN_URL = 'https://auth.x.ai/oauth2/token';
const XAI_DEVICE_AUTHORIZATION_URL = 'https://auth.x.ai/oauth2/device/code';
const XAI_DEVICE_CODE_GRANT_TYPE = 'urn:ietf:params:oauth:grant-type:device_code';
const XAI_AUTH_TIMEOUT_MS = 5 * 60 * 1000;
const XAI_AUTH_SCOPE = 'openid profile email offline_access grok-cli:access api:access';

type FileSystem = Pick<
	typeof import('node:fs'),
	'existsSync' | 'mkdirSync' | 'readFileSync' | 'renameSync' | 'rmSync' | 'writeFileSync'
>;
type Path = Pick<typeof import('node:path'), 'dirname' | 'join'>;
type Http = Pick<typeof import('node:http'), 'createServer'>;
type Crypto = Pick<typeof import('node:crypto'), 'createHash' | 'randomBytes'>;
type Fetch = typeof globalThis.fetch;

type AuthStatus = {
	authenticated: boolean;
	authPath: string;
	error?: string;
	accountID?: string;
	expiry?: string | number;
};

type TokenRecord = Record<string, unknown> & {
	access_token?: string;
	refresh_token?: string;
	id_token?: string;
	expires_in?: number | string;
	expires_at?: number | string;
	expiry?: string;
	error?: string;
	error_description?: string;
};

export interface ProviderAuthDependencies {
	fs: FileSystem;
	path: Path;
	http: Http;
	crypto: Crypto;
	fetch: Fetch;
	platform: {
		homedir: () => string;
		environment: NodeJS.ProcessEnv;
	};
	window: {
		openExternal: (url: string) => Promise<void>;
		showMessageBox: (options: {
			type: 'info';
			title: string;
			message: string;
			detail: string;
			buttons: string[];
		}) => Promise<unknown>;
	};
	ollama: {
		listModels: (baseURL: string) => Promise<{ models: Array<{ name?: string }> }>;
	};
}

export function createProviderAuth(dependencies: ProviderAuthDependencies) {
	const { crypto, fetch, fs, http, ollama, path, platform, window } = dependencies;

	function stripTrailingSlashes(url: unknown) {
		return String(url || '')
			.trim()
			.replace(/\/+$/, '');
	}

	// Mirrors comet-sdk providerbase.Endpoint: tolerates base URLs that already end in /v1.
	function openAICompatibleEndpoint(rawBaseURL: unknown, requestPath: string) {
		let baseURL = stripTrailingSlashes(rawBaseURL);
		if (!baseURL) throw new Error('Base URL is required');
		baseURL = baseURL.replace(/\/chat\/completions$/i, '');
		const suffix = requestPath.startsWith('/') ? requestPath : `/${requestPath}`;
		return baseURL.endsWith('/v1') ? `${baseURL}${suffix}` : `${baseURL}/v1${suffix}`;
	}

	function normalizeModelsBaseURL(rawBaseURL: unknown) {
		return openAICompatibleEndpoint(rawBaseURL, '/models');
	}

	async function fetchModelsFromURL(url: string, headers: Record<string, string>) {
		try {
			return await fetch(url, {
				headers,
				signal: AbortSignal.timeout(FETCH_MODELS_TIMEOUT_MS)
			});
		} catch (err) {
			if (err instanceof Error && (err.name === 'TimeoutError' || err.name === 'AbortError')) {
				throw new Error(
					`Timed out after ${FETCH_MODELS_TIMEOUT_MS / 1000}s contacting ${url}. ` +
						'Check the base URL, VPN or network access, and that the provider exposes GET /v1/models.'
				);
			}
			const message = err instanceof Error ? err.message : String(err);
			throw new Error(`Failed to reach ${url}: ${message}`);
		}
	}

	async function readModels(
		response: Response,
		message: string,
		pickModel?: (item: unknown) => unknown,
		allowArrayPayload = true
	) {
		if (!response.ok) {
			const body = await response.text();
			throw new Error(`${response.status}: ${body || response.statusText}`);
		}
		const payload: unknown = await response.json();
		const record = payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : {};
		const rawModels = Array.isArray(record.data)
			? record.data
			: allowArrayPayload && Array.isArray(payload)
				? payload
				: [];
		const result = normalizeModelFetchResult(rawModels, pickModel);
		if (result.models.length === 0) throw new Error(message);
		return result;
	}

	async function fetchOpenAIModels(baseURL: string, apiKey: string) {
		return readModels(
			await fetchModelsFromURL(normalizeModelsBaseURL(baseURL), {
				Authorization: `Bearer ${apiKey}`,
				Accept: 'application/json'
			}),
			'No models returned by provider'
		);
	}

	async function fetchAnthropicModels(baseURL: string, apiKey: string) {
		return readModels(
			await fetchModelsFromURL(normalizeModelsBaseURL(baseURL), {
				'x-api-key': apiKey,
				'anthropic-version': '2023-06-01',
				Accept: 'application/json'
			}),
			'No models returned by Anthropic',
			undefined,
			false
		);
	}

	function codexAuthPath() {
		const codexHome = String(platform.environment.CODEX_HOME || '').trim();
		return path.join(codexHome || path.join(platform.homedir(), '.codex'), 'auth.json');
	}

	function codexRedirectURI() {
		return `http://localhost:${CODEX_AUTH_CALLBACK_PORT}${CODEX_AUTH_CALLBACK_PATH}`;
	}

	function base64URLEncode(buffer: Uint8Array) {
		return Buffer.from(buffer)
			.toString('base64')
			.replace(/\+/g, '-')
			.replace(/\//g, '_')
			.replace(/=+$/g, '');
	}

	function codexCodeVerifier() {
		return base64URLEncode(crypto.randomBytes(48));
	}

	function codexCodeChallenge(verifier: string) {
		return base64URLEncode(crypto.createHash('sha256').update(verifier).digest());
	}

	function parseJWTPayload(token: unknown): Record<string, unknown> {
		const parts = String(token || '').split('.');
		if (parts.length < 2) return {};
		try {
			return JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8')) as Record<string, unknown>;
		} catch {
			return {};
		}
	}

	function codexAccountIDFromTokens(tokens: TokenRecord) {
		if (typeof tokens.account_id === 'string') return tokens.account_id;
		const accessPayload = parseJWTPayload(tokens.access_token);
		if (typeof accessPayload.account_id === 'string') return accessPayload.account_id;
		const idPayload = parseJWTPayload(tokens.id_token);
		return typeof idPayload.account_id === 'string' ? idPayload.account_id : '';
	}

	function writeCodexAuth(tokens: TokenRecord) {
		const authPath = codexAuthPath();
		fs.mkdirSync(path.dirname(authPath), { recursive: true, mode: 0o700 });
		const auth = {
			auth_mode: 'chatgpt',
			tokens: {
				access_token: tokens.access_token,
				refresh_token: tokens.refresh_token || '',
				id_token: tokens.id_token || '',
				account_id: codexAccountIDFromTokens(tokens),
				last_refresh: new Date().toISOString()
			}
		};
		const tmpPath = `${authPath}.tmp`;
		fs.writeFileSync(tmpPath, `${JSON.stringify(auth, null, 2)}\n`, { mode: 0o600 });
		fs.renameSync(tmpPath, authPath);
		return auth;
	}

	function jwtExpiresSoon(token: unknown) {
		const exp = Number(parseJWTPayload(token).exp || 0);
		return exp > 0 && exp * 1000 <= Date.now() + 30_000;
	}

	async function fetchToken(url: string, init: RequestInit) {
		const response = await fetch(url, { ...init, signal: AbortSignal.timeout(FETCH_MODELS_TIMEOUT_MS) });
		const payload = (await response.json().catch(() => ({}))) as TokenRecord;
		return { response, payload };
	}

	async function refreshCodexAuth(auth: { tokens: TokenRecord }, authPath: string) {
		const refreshToken = String(auth.tokens.refresh_token || '').trim();
		if (!refreshToken) throw new Error('Codex session expired. Sign in with ChatGPT again.');
		const { payload, response } = await fetchToken(CODEX_REFRESH_URL, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
			body: JSON.stringify({
				client_id: CODEX_CLIENT_ID,
				grant_type: 'refresh_token',
				refresh_token: refreshToken
			})
		});
		if (!response.ok || payload.error) {
			const detail = payload.error_description || payload.error || response.statusText;
			throw new Error(`Codex refresh failed: ${detail}. Sign in with ChatGPT again.`);
		}
		if (!payload.access_token) throw new Error('Codex refresh did not return an access token');
		auth.tokens.access_token = payload.access_token;
		if (payload.refresh_token) auth.tokens.refresh_token = payload.refresh_token;
		if (payload.id_token) auth.tokens.id_token = payload.id_token;
		auth.tokens.account_id = codexAccountIDFromTokens(auth.tokens);
		auth.tokens.last_refresh = new Date().toISOString();
		const tmpPath = `${authPath}.tmp`;
		fs.writeFileSync(tmpPath, `${JSON.stringify(auth, null, 2)}\n`, { mode: 0o600 });
		fs.renameSync(tmpPath, authPath);
		return auth;
	}

	async function borrowCodexAuth() {
		const authPath = codexAuthPath();
		if (!fs.existsSync(authPath)) {
			throw new Error(`Codex auth file not found at ${authPath}. Sign in with ChatGPT first.`);
		}
		let auth: { auth_mode?: string; tokens?: TokenRecord };
		try {
			auth = JSON.parse(fs.readFileSync(authPath, 'utf8')) as typeof auth;
		} catch (err) {
			throw new Error(`Failed to read Codex auth file: ${err instanceof Error ? err.message : err}`);
		}
		if (auth.auth_mode !== 'chatgpt') {
			throw new Error('Codex is not signed in with ChatGPT browser auth. Sign in with ChatGPT first.');
		}
		if (!auth.tokens?.access_token) {
			throw new Error('Codex auth file has no access token. Sign in with ChatGPT first.');
		}
		if (jwtExpiresSoon(auth.tokens.access_token)) {
			auth = await refreshCodexAuth(auth as { tokens: TokenRecord }, authPath);
		}
		const tokens = auth.tokens as TokenRecord;
		return { accessToken: String(tokens.access_token), accountID: String(tokens.account_id || '') };
	}

	function getCodexAuthStatus(): AuthStatus {
		const authPath = codexAuthPath();
		if (!fs.existsSync(authPath)) return { authenticated: false, authPath, error: 'Not signed in' };
		try {
			const auth = JSON.parse(fs.readFileSync(authPath, 'utf8')) as {
				auth_mode?: string;
				tokens?: TokenRecord;
			};
			if (auth.auth_mode !== 'chatgpt') {
				return { authenticated: false, authPath, error: 'Codex is not signed in with ChatGPT browser auth' };
			}
			if (!auth.tokens?.access_token) {
				return { authenticated: false, authPath, error: 'Codex auth file has no access token' };
			}
			return { authenticated: true, authPath, accountID: String(auth.tokens.account_id || '') || undefined };
		} catch (err) {
			return { authenticated: false, authPath, error: err instanceof Error ? err.message : String(err) };
		}
	}

	async function exchangeCodexCode(code: string, codeVerifier: string) {
		const { payload, response } = await fetchToken(CODEX_REFRESH_URL, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
			body: JSON.stringify({
				client_id: CODEX_CLIENT_ID,
				grant_type: 'authorization_code',
				code,
				redirect_uri: codexRedirectURI(),
				code_verifier: codeVerifier
			})
		});
		if (!response.ok || payload.error) {
			throw new Error(`ChatGPT sign-in failed: ${payload.error_description || payload.error || response.statusText}`);
		}
		if (!payload.access_token) throw new Error('ChatGPT sign-in did not return an access token');
		return payload;
	}

	function codexAuthorizeURL(state: string, codeChallenge: string) {
		const params = new URLSearchParams({
			response_type: 'code', client_id: CODEX_CLIENT_ID, redirect_uri: codexRedirectURI(),
			scope: 'openid profile email offline_access', code_challenge: codeChallenge,
			code_challenge_method: 'S256', id_token_add_organizations: 'true',
			codex_cli_simplified_flow: 'true', state, originator: 'cometline'
		});
		return `https://auth.openai.com/oauth/authorize?${params.toString()}`;
	}

	type CallbackOptions = {
		port: number;
		callbackPath: string;
		redirectURI: string;
		state: string;
		authorizeURL: string;
		timeoutMessage: string;
		failureHeading: string;
		successHTML: string;
		browserError: string;
	};

	async function waitForOAuthCode(options: CallbackOptions) {
		let server: ReturnType<Http['createServer']> | undefined;
		try {
			return await new Promise<string>((resolve, reject) => {
				const timeout = setTimeout(() => reject(new Error(options.timeoutMessage)), CODEX_AUTH_TIMEOUT_MS);
				server = http.createServer((req, res) => {
					const requestURL = new URL(req.url || '/', options.redirectURI);
					if (requestURL.pathname !== options.callbackPath) {
						res.writeHead(404, { 'Content-Type': 'text/plain' });
						res.end('Not found');
						return;
					}
					const error = requestURL.searchParams.get('error');
					const returnedState = requestURL.searchParams.get('state');
					const returnedCode = requestURL.searchParams.get('code');
					if (returnedState !== options.state) {
						res.writeHead(400, { 'Content-Type': 'text/html' });
						res.end(`<h1>${options.failureHeading}</h1><p>Invalid OAuth state.</p>`);
						clearTimeout(timeout);
						reject(new Error(`${options.failureHeading}: invalid OAuth state.`));
						return;
					}
					if (error) {
						res.writeHead(400, { 'Content-Type': 'text/html' });
						res.end(`<h1>${options.failureHeading}</h1><p>${error}</p>`);
						clearTimeout(timeout);
						reject(new Error(`${options.failureHeading}: ${error}`));
						return;
					}
					if (!returnedCode) {
						res.writeHead(400, { 'Content-Type': 'text/html' });
						res.end(`<h1>${options.failureHeading}</h1><p>No authorization code returned.</p>`);
						clearTimeout(timeout);
						reject(new Error(`${options.failureHeading}: no authorization code returned.`));
						return;
					}
					res.writeHead(200, { 'Content-Type': 'text/html' });
					res.end(options.successHTML);
					clearTimeout(timeout);
					resolve(returnedCode);
				});
				server.once('error', (err) => { clearTimeout(timeout); reject(err); });
				server.listen(options.port, async () => {
					try {
						await window.openExternal(options.authorizeURL);
					} catch (err) {
						clearTimeout(timeout);
						reject(new Error(`${options.browserError}: ${err instanceof Error ? err.message : err}`));
					}
				});
			});
		} finally {
			if (server) server.close();
		}
	}

	async function startCodexLogin() {
		const state = base64URLEncode(crypto.randomBytes(32));
		const codeVerifier = codexCodeVerifier();
		const code = await waitForOAuthCode({
			port: CODEX_AUTH_CALLBACK_PORT, callbackPath: CODEX_AUTH_CALLBACK_PATH, redirectURI: codexRedirectURI(),
			state, authorizeURL: codexAuthorizeURL(state, codexCodeChallenge(codeVerifier)),
			timeoutMessage: 'Timed out waiting for ChatGPT sign-in to complete.',
			failureHeading: 'ChatGPT sign-in failed',
			successHTML: '<h1>Signed in with ChatGPT</h1><p>You can return to Cometline.</p>',
			browserError: 'Failed to open ChatGPT sign-in in your browser'
		});
		writeCodexAuth(await exchangeCodexCode(code, codeVerifier));
		return { started: true, message: 'Signed in with ChatGPT. You can fetch Codex models now.' };
	}

	function xaiAuthPath() {
		const home = String(platform.environment.XAI_HOME || '').trim();
		return path.join(home || path.join(platform.homedir(), '.cometmind', 'xai'), 'auth.json');
	}

	function xaiAuthExpiresSoon(token: unknown, expiresAt: unknown) {
		if (Number(expiresAt) > 0 && Number(expiresAt) <= Date.now() + 2 * 60 * 1000) return true;
		const exp = Number(parseJWTPayload(token).exp || 0);
		return exp > 0 && exp * 1000 <= Date.now() + 2 * 60 * 1000;
	}

	function writeXaiAuth(tokens: TokenRecord) {
		const authPath = xaiAuthPath();
		fs.mkdirSync(path.dirname(authPath), { recursive: true, mode: 0o700 });
		const expiresAt = Number(tokens.expires_in) > 0 ? Date.now() + Number(tokens.expires_in) * 1000 : 0;
		const auth = {
			auth_mode: 'subscription',
			tokens: {
				access_token: tokens.access_token, refresh_token: tokens.refresh_token || '', expires_at: expiresAt,
				last_refresh: new Date().toISOString()
			}
		};
		const tmpPath = `${authPath}.tmp`;
		fs.writeFileSync(tmpPath, `${JSON.stringify(auth, null, 2)}\n`, { mode: 0o600 });
		fs.renameSync(tmpPath, authPath);
		return auth;
	}

	async function refreshXaiAuth(auth: { tokens: TokenRecord }, authPath: string) {
		const refreshToken = String(auth.tokens.refresh_token || '').trim();
		if (!refreshToken) throw new Error('Grok subscription session expired. Sign in with Grok again.');
		const { payload, response } = await fetchToken(XAI_TOKEN_URL, {
			method: 'POST',
			headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json', 'User-Agent': 'cometline' },
			body: new URLSearchParams({ client_id: XAI_CLIENT_ID, grant_type: 'refresh_token', refresh_token: refreshToken }).toString()
		});
		if (!response.ok || payload.error) {
			throw new Error(`Grok refresh failed: ${payload.error_description || payload.error || response.statusText}. Sign in with Grok again.`);
		}
		if (!payload.access_token) throw new Error('Grok refresh did not return an access token');
		auth.tokens.access_token = payload.access_token;
		if (payload.refresh_token) auth.tokens.refresh_token = payload.refresh_token;
		auth.tokens.expires_at = Number(payload.expires_in) > 0 ? Date.now() + Number(payload.expires_in) * 1000 : 0;
		auth.tokens.last_refresh = new Date().toISOString();
		const tmpPath = `${authPath}.tmp`;
		fs.writeFileSync(tmpPath, `${JSON.stringify(auth, null, 2)}\n`, { mode: 0o600 });
		fs.renameSync(tmpPath, authPath);
		return auth;
	}

	async function withXaiAuthLock<T>(authPath: string, fn: () => Promise<T>) {
		const lockPath = `${authPath}.lock`;
		const deadline = Date.now() + 30_000;
		fs.mkdirSync(path.dirname(authPath), { recursive: true, mode: 0o700 });
		while (true) {
			try {
				fs.mkdirSync(lockPath, { mode: 0o700 });
				break;
			} catch (err) {
				if (!(err instanceof Error && 'code' in err && err.code === 'EEXIST') || Date.now() >= deadline) {
					throw new Error(`Failed to acquire Grok auth lock: ${err instanceof Error ? err.message : err}`);
				}
				await new Promise((resolve) => setTimeout(resolve, 50));
			}
		}
		try { return await fn(); } finally { fs.rmSync(lockPath, { recursive: true, force: true }); }
	}

	async function borrowXaiAuth() {
		const authPath = xaiAuthPath();
		return withXaiAuthLock(authPath, async () => {
			if (!fs.existsSync(authPath)) throw new Error(`Grok subscription session not found at ${authPath}. Sign in with Grok first.`);
			let auth: { auth_mode?: string; tokens?: TokenRecord };
			try { auth = JSON.parse(fs.readFileSync(authPath, 'utf8')) as typeof auth; }
			catch (err) { throw new Error(`Failed to read Grok auth file: ${err instanceof Error ? err.message : err}`); }
			if (auth.auth_mode !== 'subscription') throw new Error('Grok auth file is not a subscription session. Sign in with Grok first.');
			if (!auth.tokens?.access_token) throw new Error('Grok auth file has no access token. Sign in with Grok first.');
			if (xaiAuthExpiresSoon(auth.tokens.access_token, auth.tokens.expires_at)) {
				auth = await refreshXaiAuth(auth as { tokens: TokenRecord }, authPath);
			}
			return { accessToken: String((auth.tokens as TokenRecord).access_token) };
		});
	}

	function getXaiAuthStatus(): AuthStatus {
		const authPath = xaiAuthPath();
		if (!fs.existsSync(authPath)) return { authenticated: false, authPath, error: 'Not signed in' };
		try {
			const auth = JSON.parse(fs.readFileSync(authPath, 'utf8')) as { auth_mode?: string; tokens?: TokenRecord };
			if (auth.auth_mode !== 'subscription') return { authenticated: false, authPath, error: 'Grok auth file is not a subscription session' };
			if (!auth.tokens?.access_token) return { authenticated: false, authPath, error: 'Grok auth file has no access token' };
			return { authenticated: true, authPath };
		} catch (err) {
			return { authenticated: false, authPath, error: err instanceof Error ? err.message : String(err) };
		}
	}

	async function requestXaiDeviceCode() {
		const { payload, response } = await fetchToken(XAI_DEVICE_AUTHORIZATION_URL, {
			method: 'POST',
			headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json', 'User-Agent': 'cometline' },
			body: new URLSearchParams({ client_id: XAI_CLIENT_ID, scope: XAI_AUTH_SCOPE }).toString()
		});
		if (!response.ok || payload.error) throw new Error(`Could not start Grok device sign-in: ${payload.error_description || payload.error || response.statusText}`);
		if (!payload.device_code || !payload.user_code || !payload.verification_uri) throw new Error('Grok device sign-in returned incomplete authorization details.');
		return payload;
	}

	async function pollXaiDeviceCode(device: TokenRecord) {
		const expiresInMs = Number.isFinite(Number(device.expires_in)) && Number(device.expires_in) > 0 ? Number(device.expires_in) * 1000 : XAI_AUTH_TIMEOUT_MS;
		const deadline = Date.now() + expiresInMs;
		let intervalMs = Math.max(Number.isFinite(Number(device.interval)) && Number(device.interval) > 0 ? Number(device.interval) * 1000 : 5000, 1000);
		while (Date.now() < deadline) {
			const { payload, response } = await fetchToken(XAI_TOKEN_URL, {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json', 'User-Agent': 'cometline' },
				body: new URLSearchParams({ client_id: XAI_CLIENT_ID, grant_type: XAI_DEVICE_CODE_GRANT_TYPE, device_code: String(device.device_code) }).toString()
			});
			if (response.ok && payload.access_token) return payload;
			if (payload.error === 'authorization_pending') { await new Promise((resolve) => setTimeout(resolve, intervalMs)); continue; }
			if (payload.error === 'slow_down') { intervalMs += 5000; await new Promise((resolve) => setTimeout(resolve, intervalMs)); continue; }
			if (payload.error === 'access_denied' || payload.error === 'authorization_denied') throw new Error('Grok device authorization was denied.');
			if (payload.error === 'expired_token') throw new Error('Grok device authorization expired. Try again.');
			throw new Error(`Grok device sign-in failed: ${payload.error_description || payload.error || response.statusText}`);
		}
		throw new Error('Grok device authorization timed out. Try again.');
	}

	async function startXaiLogin() {
		const device = await requestXaiDeviceCode();
		const verificationURL = String(device.verification_uri_complete || device.verification_uri);
		try { await window.openExternal(verificationURL); }
		catch (err) { throw new Error(`Failed to open Grok device sign-in in your browser: ${err instanceof Error ? err.message : err}`); }
		await window.showMessageBox({
			type: 'info', title: 'Finish signing in with Grok', message: 'Complete the device sign-in in your browser.',
			detail: `If the code is not pre-filled, enter this code:\n\n${device.user_code}\n\nVerification URL:\n${device.verification_uri}`,
			buttons: ['Continue waiting']
		});
		writeXaiAuth(await pollXaiDeviceCode(device));
		return { started: true, message: 'Signed in with Grok subscription. You can fetch xAI models now.' };
	}

	function readCursorMcpConfig() {
		const filePath = path.join(platform.homedir(), '.cursor', 'mcp.json');
		if (!fs.existsSync(filePath)) return { ok: false, error: 'Cursor MCP config not found at ~/.cursor/mcp.json' };
		try { return { ok: true, path: filePath, config: JSON.parse(fs.readFileSync(filePath, 'utf8')) }; }
		catch (err) { return { ok: false, error: err instanceof Error ? err.message : 'Failed to read Cursor MCP config' }; }
	}

	function normalizeModelFetchResult(rawModels: unknown[], pickModel: (item: Record<string, unknown>) => unknown = (item) => item.id) {
		const models: string[] = [];
		for (const item of rawModels) {
			if (typeof item === 'string') { const id = item.trim(); if (id) models.push(id); continue; }
			if (!item || typeof item !== 'object') continue;
			const id = String(pickModel(item as Record<string, unknown>) || '').trim();
			if (id) models.push(id);
		}
		return { models: Array.from(new Set(models)).sort() };
	}

	async function fetchCodexModels(baseURL: string) {
		const auth = await borrowCodexAuth();
		const headers: Record<string, string> = { Authorization: `Bearer ${auth.accessToken}`, Accept: 'application/json' };
		if (auth.accountID) headers['ChatGPT-Account-ID'] = auth.accountID;
		const response = await fetchModelsFromURL(`${String(baseURL || CODEX_BASE_URL).replace(/\/+$/, '')}/models?client_version=${encodeURIComponent(CODEX_CLIENT_VERSION)}`, headers);
		if (!response.ok) { const body = await response.text(); throw new Error(`${response.status}: ${body || response.statusText}`); }
		const payload = (await response.json()) as Record<string, unknown>;
		const rawModels = Array.isArray(payload.models) ? payload.models : Array.isArray(payload.data) ? payload.data : [];
		const filtered = rawModels.filter((item) => typeof item === 'string' || (item && typeof item === 'object' && (item as Record<string, unknown>).supported_in_api !== false && (item as Record<string, unknown>).visibility !== 'hidden'));
		const result = normalizeModelFetchResult(filtered, (item) => item.slug || item.id);
		if (result.models.length === 0) throw new Error('No models returned by Codex');
		return result;
	}

	async function fetchXaiModels(baseURL: string) {
		const auth = await borrowXaiAuth();
		return readModels(
			await fetchModelsFromURL(normalizeModelsBaseURL(baseURL || XAI_BASE_URL), { Authorization: `Bearer ${auth.accessToken}`, Accept: 'application/json', 'User-Agent': 'cometline' }),
			'No models returned by xAI'
		);
	}

	async function fetchOpenCodeGoModels(baseURL: string) {
		return readModels(await fetchModelsFromURL(normalizeModelsBaseURL(baseURL || 'https://opencode.ai/zen/go/v1'), { Accept: 'application/json' }), 'No models returned by OpenCode Go');
	}

	async function fetchProviderModels(config: Pick<ProviderConfig, 'method' | 'baseURL' | 'apiKey'>) {
		if (config.method === 'opencode-go') return fetchOpenCodeGoModels(config.baseURL);
		if (config.method === 'codex') return fetchCodexModels(config.baseURL);
		if (config.method === 'xai') return fetchXaiModels(config.baseURL);
		if (config.method === 'ollama') {
			const models = (await ollama.listModels(config.baseURL)).models.map((model) => model.name).filter((name): name is string => Boolean(name));
			if (models.length === 0) throw new Error('No models installed in Ollama yet. Pull a model first.');
			return { models };
		}
		const baseURL = String(config.baseURL || '').trim();
		const apiKey = String(config.apiKey || '').trim();
		if (!baseURL) throw new Error('Base URL is required');
		if (!apiKey) throw new Error('API key is required');
		return config.method === 'anthropic' ? fetchAnthropicModels(baseURL, apiKey) : fetchOpenAIModels(baseURL, apiKey);
	}

	return {
		fetchProviderModels,
		getCodexAuthStatus,
		startCodexLogin,
		getXaiAuthStatus,
		startXaiLogin,
		readCursorMcpConfig
	};
}
