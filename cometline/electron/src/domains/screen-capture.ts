import { desktopCapturer, screen, systemPreferences, type DesktopCapturerSource } from 'electron';
import crypto from 'node:crypto';
import http from 'node:http';

const CAPTURE_TIMEOUT_MS = 15_000;
const MAX_CAPTURE_EDGE = 2560;
const DEFAULT_CAPTURE_EDGE = 1920;
const JPEG_QUALITY = 82;

export interface ScreenCaptureBridge {
	start(): Promise<void>;
	stop(): Promise<void>;
	getEnvironment(): NodeJS.ProcessEnv;
}

type CaptureRequest = {
	display?: unknown;
	sourceId?: unknown;
	window?: unknown;
	maxWidth?: unknown;
	maxHeight?: unknown;
	crop?: {
		x?: unknown;
		y?: unknown;
		width?: unknown;
		height?: unknown;
	};
};

function readBody(req: http.IncomingMessage): Promise<string> {
	return new Promise((resolve, reject) => {
		let body = '';
		req.setEncoding('utf8');
		req.on('data', (chunk: string) => {
			body += chunk;
			if (body.length > 16 * 1024) {
				req.destroy();
				reject(new Error('request body too large'));
			}
		});
		req.once('end', () => resolve(body));
		req.once('error', reject);
	});
}

function sendJson(res: http.ServerResponse, status: number, payload: unknown) {
	res.statusCode = status;
	res.setHeader('Content-Type', 'application/json; charset=utf-8');
	res.setHeader('Cache-Control', 'no-store');
	res.end(JSON.stringify(payload));
}

function clampEdge(value: unknown, fallback: number): number {
	const n = Number(value);
	if (!Number.isFinite(n) || n <= 0) return fallback;
	return Math.min(MAX_CAPTURE_EDGE, Math.max(320, Math.floor(n)));
}

function screenAccessDenied(): boolean {
	if (process.platform !== 'darwin') return false;
	try {
		const status = systemPreferences.getMediaAccessStatus('screen');
		return status === 'denied' || status === 'restricted';
	} catch {
		return false;
	}
}

function assertScreenAccess() {
	if (screenAccessDenied()) {
		throw new Error(
			'Screen recording permission is denied. Enable Cometline in System Settings → Screen & System Audio Recording.'
		);
	}
}

function sourceKind(id: string): 'screen' | 'window' {
	return id.startsWith('screen:') ? 'screen' : 'window';
}

async function getSources(
	types: Array<'screen' | 'window'>,
	thumbnailSize: { width: number; height: number }
): Promise<DesktopCapturerSource[]> {
	return Promise.race([
		desktopCapturer.getSources({
			types,
			thumbnailSize,
			fetchWindowIcons: false
		}),
		new Promise<never>((_, reject) =>
			setTimeout(() => reject(new Error('screen capture timed out')), CAPTURE_TIMEOUT_MS)
		)
	]);
}

async function listCaptureTargets(): Promise<{
	sources: Array<{
		id: string;
		name: string;
		type: 'screen' | 'window';
		display_id: string;
	}>;
}> {
	assertScreenAccess();
	const sources = await getSources(['screen', 'window'], { width: 1, height: 1 });
	return {
		sources: sources.map((source) => ({
			id: source.id,
			name: source.name,
			type: sourceKind(source.id),
			display_id: String(source.display_id || '')
		}))
	};
}

function resolveThumbnailSize(input: CaptureRequest): { width: number; height: number } {
	const displays = screen.getAllDisplays();
	const displayIndex = Math.max(0, Math.floor(Number(input.display) || 0));
	const display = displays[Math.min(displayIndex, Math.max(displays.length - 1, 0))] ?? displays[0];
	if (!display) {
		return { width: DEFAULT_CAPTURE_EDGE, height: DEFAULT_CAPTURE_EDGE };
	}
	const scale = Math.max(1, display.scaleFactor || 1);
	const nativeWidth = Math.floor(display.size.width * scale);
	const nativeHeight = Math.floor(display.size.height * scale);
	const maxWidth = clampEdge(
		input.maxWidth,
		Math.min(DEFAULT_CAPTURE_EDGE, MAX_CAPTURE_EDGE, nativeWidth)
	);
	const maxHeight = clampEdge(
		input.maxHeight,
		Math.min(DEFAULT_CAPTURE_EDGE, MAX_CAPTURE_EDGE, nativeHeight)
	);
	return {
		width: Math.min(nativeWidth, maxWidth),
		height: Math.min(nativeHeight, maxHeight)
	};
}

function applyCrop(
	image: Electron.NativeImage,
	crop: CaptureRequest['crop']
): Electron.NativeImage {
	if (!crop) return image;
	const size = image.getSize();
	const x = Math.max(0, Math.floor(Number(crop.x) || 0));
	const y = Math.max(0, Math.floor(Number(crop.y) || 0));
	const width = Math.floor(Number(crop.width) || 0);
	const height = Math.floor(Number(crop.height) || 0);
	if (width <= 0 || height <= 0) {
		throw new Error('crop.width and crop.height must be positive');
	}
	if (x >= size.width || y >= size.height) {
		throw new Error('crop origin is outside the captured image');
	}
	const rect = {
		x,
		y,
		width: Math.min(width, size.width - x),
		height: Math.min(height, size.height - y)
	};
	return image.crop(rect);
}

async function capture(input: CaptureRequest): Promise<{
	media_type: 'image/jpeg';
	data: string;
	width: number;
	height: number;
	display_id: string;
	source_id: string;
	source_name: string;
	source_type: 'screen' | 'window';
}> {
	assertScreenAccess();
	const thumbnailSize = resolveThumbnailSize(input);
	const sourceId = String(input.sourceId || '').trim();
	const windowQuery = String(input.window || '').trim().toLowerCase();
	const wantWindow = Boolean(sourceId || windowQuery);

	const sources = await getSources(wantWindow ? ['screen', 'window'] : ['screen'], thumbnailSize);

	let match: DesktopCapturerSource | undefined;
	if (sourceId) {
		match = sources.find((source) => source.id === sourceId);
		if (!match) {
			throw new Error(`no capture source with id ${sourceId}`);
		}
	} else if (windowQuery) {
		const windows = sources.filter((source) => sourceKind(source.id) === 'window');
		match =
			windows.find((source) => source.name.toLowerCase() === windowQuery) ??
			windows.find((source) => source.name.toLowerCase().includes(windowQuery));
		if (!match) {
			const sample = windows
				.slice(0, 12)
				.map((source) => source.name)
				.join('; ');
			throw new Error(
				`no window matching ${JSON.stringify(input.window)}${sample ? ` (examples: ${sample})` : ''}`
			);
		}
	} else {
		const displays = screen.getAllDisplays();
		if (displays.length === 0) {
			throw new Error('no displays available');
		}
		const displayIndex = Math.max(0, Math.floor(Number(input.display) || 0));
		const display = displays[Math.min(displayIndex, displays.length - 1)] ?? displays[0];
		match =
			sources.find((source) => String(source.display_id) === String(display.id)) ?? sources[0];
	}

	if (!match) {
		throw new Error('no screen capture source available');
	}

	let image = match.thumbnail;
	if (image.isEmpty()) {
		throw new Error(
			'captured image was empty — grant Screen & System Audio Recording to Cometline and try again'
		);
	}
	image = applyCrop(image, input.crop);
	const jpeg = image.toJPEG(JPEG_QUALITY);
	if (!jpeg || jpeg.length === 0) {
		throw new Error('captured image was empty after encoding');
	}
	const size = image.getSize();
	return {
		media_type: 'image/jpeg',
		data: jpeg.toString('base64'),
		width: size.width,
		height: size.height,
		display_id: String(match.display_id || ''),
		source_id: match.id,
		source_name: match.name,
		source_type: sourceKind(match.id)
	};
}

export function createScreenCaptureBridge(options?: {
	isPreferred?: () => boolean;
}): ScreenCaptureBridge {
	let server: http.Server | null = null;
	let endpoint = '';
	let token = '';
	let queue: Promise<unknown> = Promise.resolve();

	function hasValidToken(value: string | string[] | undefined) {
		return typeof value === 'string' && value.length > 0 && value === token;
	}

	function enqueue<T>(task: () => Promise<T>): Promise<T> {
		const run = queue.then(task, task);
		queue = run.catch(() => undefined);
		return run;
	}

	function guardPreferred(res: http.ServerResponse): boolean {
		if (options?.isPreferred && !options.isPreferred()) {
			sendJson(res, 403, {
				error:
					'Screen capture is disabled in Settings → App. Enable Screen & system audio first.'
			});
			return false;
		}
		return true;
	}

	return {
		async start() {
			if (server) return;
			token = crypto.randomBytes(32).toString('hex');
			server = http.createServer(async (req, res) => {
				const url = req.url || '';
				if (!hasValidToken(req.headers['x-cometline-screen-capture-token'])) {
					return sendJson(res, 401, { error: 'unauthorized' });
				}
				if (req.method === 'GET' && url === '/sources') {
					if (!guardPreferred(res)) return;
					try {
						const payload = await enqueue(() => listCaptureTargets());
						sendJson(res, 200, payload);
					} catch (error) {
						sendJson(res, 502, {
							error: error instanceof Error ? error.message : String(error)
						});
					}
					return;
				}
				if (req.method === 'POST' && url === '/capture') {
					if (!guardPreferred(res)) return;
					try {
						const raw = await readBody(req);
						const input = (raw ? JSON.parse(raw) : {}) as CaptureRequest;
						const payload = await enqueue(() => capture(input));
						sendJson(res, 200, payload);
					} catch (error) {
						sendJson(res, 502, {
							error: error instanceof Error ? error.message : String(error)
						});
					}
					return;
				}
				return sendJson(res, 404, { error: 'not_found' });
			});
			await new Promise<void>((resolve, reject) => {
				server?.once('error', reject);
				server?.listen(0, '127.0.0.1', () => {
					const address = server?.address();
					if (!address || typeof address === 'string') {
						return reject(new Error('screen capture bridge did not expose a TCP port'));
					}
					endpoint = `http://127.0.0.1:${address.port}/capture`;
					resolve();
				});
			});
		},
		async stop() {
			endpoint = '';
			token = '';
			if (!server) return;
			const activeServer = server;
			server = null;
			await new Promise<void>((resolve) => activeServer.close(() => resolve()));
		},
		getEnvironment() {
			return endpoint && token
				? {
						COMETLINE_SCREEN_CAPTURE_URL: endpoint,
						COMETLINE_SCREEN_CAPTURE_TOKEN: token
					}
				: {};
		}
	};
}
