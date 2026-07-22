import type { Net, Protocol } from 'electron';
import type path from 'node:path';
import { pathToFileURL } from 'node:url';

export const APP_SCHEME = 'app';
export const APP_HOST = 'bundle';
export const APP_ORIGIN = `${APP_SCHEME}://${APP_HOST}`;

interface AppProtocolDependencies {
	bundleDirectory: string;
	fs: {
		existsSync(filePath: string): boolean;
		statSync(filePath: string): { isFile(): boolean };
	};
	net: Pick<Net, 'fetch'>;
	path: Pick<typeof path, 'join' | 'normalize' | 'sep'>;
	protocol: Pick<Protocol, 'handle'>;
}

/** Serves the packaged SPA without allowing requests to escape its build directory. */
export function registerAppProtocol(dependencies: AppProtocolDependencies) {
	const { bundleDirectory, fs, net, path, protocol } = dependencies;
	const fallback = path.join(bundleDirectory, 'index.html');

	protocol.handle(APP_SCHEME, (request) => {
		const requestUrl = new URL(request.url);
		let relativePath = decodeURIComponent(requestUrl.pathname).replace(/^\/+/, '');
		if (!relativePath) relativePath = 'index.html';

		let resolved = path.normalize(path.join(bundleDirectory, relativePath));
		const withinBundle =
			resolved === bundleDirectory || resolved.startsWith(`${bundleDirectory}${path.sep}`);
		if (!withinBundle) {
			return new Response('Forbidden', { status: 403 });
		}

		if (!fs.existsSync(resolved) || !fs.statSync(resolved).isFile()) {
			resolved = fallback;
		}

		return net.fetch(pathToFileURL(resolved).toString());
	});
}
