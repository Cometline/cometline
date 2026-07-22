import path from 'node:path';
import { describe, expect, it, vi } from 'vitest';

import { APP_SCHEME, registerAppProtocol } from './app-protocol.js';

describe('app protocol', () => {
	it('serves bundle files, falls back to the SPA shell, and rejects traversal', async () => {
		let handler: ((request: { url: string }) => Response | Promise<Response>) | undefined;
		const fetch = vi.fn(async () => new Response('bundle'));
		registerAppProtocol({
			bundleDirectory: '/bundle',
			fs: {
				existsSync: (filePath) => filePath === '/bundle/_app/app.js',
				statSync: () => ({ isFile: () => true })
			},
			net: { fetch },
			path,
			protocol: {
				handle: (_scheme, nextHandler) => {
					handler = nextHandler as unknown as (request: {
						url: string;
					}) => Response | Promise<Response>;
				}
			}
		});

		expect(handler).toBeDefined();
		expect(await handler!({ url: `${APP_SCHEME}://bundle/_app/app.js` })).toBeInstanceOf(
			Response
		);
		expect(fetch).toHaveBeenCalledWith('file:///bundle/_app/app.js');

		await handler!({ url: `${APP_SCHEME}://bundle/session/example` });
		expect(fetch).toHaveBeenLastCalledWith('file:///bundle/index.html');

		const forbidden = await handler!({ url: `${APP_SCHEME}://bundle/%2e%2e%2fsecret` });
		expect(forbidden.status).toBe(403);
	});
});
