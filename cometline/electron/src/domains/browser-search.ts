import { app, BrowserWindow, type BrowserWindowConstructorOptions } from 'electron';
import crypto from 'node:crypto';
import http from 'node:http';

const BROWSER_SEARCH_MAX_QUERY = 500;
const BROWSER_SEARCH_MAX_LIMIT = 10;
const BROWSER_SEARCH_TIMEOUT_MS = 25_000;

export interface BrowserSearchBridge {
	start(): Promise<void>;
	stop(): Promise<void>;
	getEnvironment(): NodeJS.ProcessEnv;
}

export function createBrowserSearchBridge(): BrowserSearchBridge {
	let server: http.Server | null = null;
	let browserWindow: BrowserWindow | null = null;
	let endpoint = '';
	let token = '';
	let queue: Promise<unknown> = Promise.resolve();

	function sendJson(res: http.ServerResponse, status: number, payload: unknown) {
		res.statusCode = status;
		res.setHeader('Content-Type', 'application/json; charset=utf-8');
		res.setHeader('Cache-Control', 'no-store');
		res.end(JSON.stringify(payload));
	}

	function readBody(req: http.IncomingMessage): Promise<string> {
		return new Promise((resolve, reject) => {
			let body = '';
			req.setEncoding('utf8');
			req.on('data', (chunk: string) => {
				body += chunk;
				if (body.length > 64 * 1024) {
					req.destroy();
					reject(new Error('request body too large'));
				}
			});
			req.once('end', () => resolve(body));
			req.once('error', reject);
		});
	}

	function hasValidToken(value: string | string[] | undefined) {
		return typeof value === 'string' && value.length > 0 && value === token;
	}

	function enqueue<T>(task: () => Promise<T>): Promise<T> {
		const run = queue.then(task, task);
		queue = run.catch(() => undefined);
		return run;
	}

	async function search({ query, limit, recency }: { query: string; limit: number; recency?: unknown }) {
		return enqueue(async () => {
			if (!browserWindow || browserWindow.isDestroyed()) {
				browserWindow = new BrowserWindow({
					width: 1280,
					height: 900,
					show: false,
					offscreen: true,
					webPreferences: {
						contextIsolation: true,
						nodeIntegration: false,
						sandbox: true,
						devTools: !app.isPackaged
					}
				} as BrowserWindowConstructorOptions);
				browserWindow.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
				browserWindow.on('closed', () => {
					browserWindow = null;
				});
			}

			const dateFilter = { day: 'd', d: 'd', week: 'w', w: 'w', month: 'm', m: 'm', year: 'y', y: 'y' }[
				String(recency || '').toLowerCase()
			];
			const searchParams = new URLSearchParams({
				q: query,
				hl: 'en',
				num: String(Math.min(20, Math.max(limit * 2, 10)))
			});
			if (dateFilter) searchParams.set('tbs', `qdr:${dateFilter}`);
			await Promise.race([
				browserWindow.loadURL(`https://www.google.com/search?${searchParams.toString()}`, {
					extraHeaders: 'Accept-Language: en-US,en;q=0.8\r\n'
				}),
				new Promise((_, reject) =>
					setTimeout(() => reject(new Error('browser search timed out')), BROWSER_SEARCH_TIMEOUT_MS)
				)
			]);

			const result = await browserWindow.webContents.executeJavaScript(
				`(() => {
					const limit = ${JSON.stringify(limit)};
					const normalize = (raw) => {
						try {
							const parsed = new URL(raw, location.href);
							if (parsed.hostname === location.hostname && parsed.pathname === '/url') {
								const redirected = parsed.searchParams.get('q') || parsed.searchParams.get('url');
								if (redirected) return new URL(redirected, location.href).href;
							}
							return parsed.href;
						} catch { return ''; }
					};
					const bodyText = document.body?.innerText || '';
					const blocked = location.hostname === 'consent.google.com' || /Before you continue to Google|unusual traffic|not a robot/i.test(bodyText);
					const seen = new Set();
					const results = [...document.querySelectorAll('a')].map((anchor) => {
						const heading = anchor.querySelector('h3');
						if (!heading) return null;
						const card = anchor.closest('div.MjjYud') || anchor.closest('div.g') || anchor.parentElement;
						const url = normalize(anchor.href);
						let source = '';
						try { source = new URL(url).hostname.replace(/^www\\./, ''); } catch { /* ignore */ }
						return { title: (heading.innerText || heading.textContent || '').trim(), url, snippet: (card?.querySelector('.VwiC3b, .yXK7lf, [data-sncf], .kb0PBd')?.innerText || '').trim(), source };
					}).filter((item) => {
						if (!item?.title || !item.url || seen.has(item.url)) return false;
						try { if (!/^https?:$/.test(new URL(item.url).protocol)) return false; } catch { return false; }
						seen.add(item.url);
						return true;
					}).slice(0, limit);
					return { blocked, results };
				})()`
			) as { blocked?: boolean; results?: unknown[] };
			if (result?.blocked && (!Array.isArray(result?.results) || result.results.length === 0)) {
				throw new Error('Google Search requires consent or is temporarily unavailable');
			}
			return { query, backend: 'electron-chromium-google', results: Array.isArray(result?.results) ? result.results : [] };
		});
	}

	return {
		async start() {
			if (server) return;
			token = crypto.randomBytes(32).toString('hex');
			server = http.createServer(async (req, res) => {
				if (req.method !== 'POST' || req.url !== '/search') return sendJson(res, 404, { error: 'not_found' });
				if (!hasValidToken(req.headers['x-cometline-browser-token'])) return sendJson(res, 401, { error: 'unauthorized' });
				try {
					const input = JSON.parse(await readBody(req)) as { query?: unknown; limit?: unknown; recency?: unknown };
					const query = String(input?.query || '').trim();
					const limit = Math.min(BROWSER_SEARCH_MAX_LIMIT, Math.max(1, Number.isFinite(Number(input?.limit)) ? Number(input.limit) : 5));
					if (!query || query.length > BROWSER_SEARCH_MAX_QUERY) return sendJson(res, 400, { error: 'invalid_query' });
					sendJson(res, 200, await search({ query, limit, recency: input?.recency }));
				} catch (error) {
					sendJson(res, 502, { error: error instanceof Error ? error.message : String(error) });
				}
			});
			await new Promise<void>((resolve, reject) => {
				server?.once('error', reject);
				server?.listen(0, '127.0.0.1', () => {
					const address = server?.address();
					if (!address || typeof address === 'string') return reject(new Error('browser search bridge did not expose a TCP port'));
					endpoint = `http://127.0.0.1:${address.port}/search`;
					resolve();
				});
			});
		},
		async stop() {
			if (browserWindow && !browserWindow.isDestroyed()) browserWindow.destroy();
			browserWindow = null;
			endpoint = '';
			token = '';
			if (!server) return;
			const activeServer = server;
			server = null;
			await new Promise<void>((resolve) => activeServer.close(() => resolve()));
		},
		getEnvironment() {
			return endpoint && token
				? { COMETLINE_BROWSER_SEARCH_URL: endpoint, COMETLINE_BROWSER_SEARCH_TOKEN: token }
				: {};
		}
	};
}
