// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-nocheck
// Runtime validation is intentional here: Ollama's streamed JSON is untyped.
const DEFAULT_NATIVE_BASE = 'http://127.0.0.1:11434';
const OLLAMA_MODEL_NAME_RE = /^[a-zA-Z0-9]([a-zA-Z0-9._:-]{0,198}[a-zA-Z0-9])?$/;
const MAX_LINE_BYTES = 64 * 1024;
const PULL_TIMEOUT_MS = 60 * 60 * 1000;

/** Keep in sync with cometline/src/lib/ollama/catalog.ts */
const OLLAMA_CATALOG = [
	{ id: 'private-memory', pullName: 'qwen3-embedding:0.6b' },
	{ id: 'local-companion-efficient', pullName: 'gemma4:e2b-mlx' },
	{ id: 'local-companion-better', pullName: 'gemma4:e4b-mlx' },
	{ id: 'private-memory-lightweight', pullName: 'embeddinggemma' }
];

function normalizeNativeBase(raw) {
	let base = String(raw || '')
		.trim()
		.replace(/\/+$/, '');
	if (!base) return DEFAULT_NATIVE_BASE;
	if (base.toLowerCase().endsWith('/v1')) {
		base = base.slice(0, -3).replace(/\/+$/, '');
	}
	return base || DEFAULT_NATIVE_BASE;
}

function isLoopbackURL(raw) {
	try {
		const parsed = new URL(normalizeNativeBase(raw));
		const host = parsed.hostname.toLowerCase();
		return host === '127.0.0.1' || host === 'localhost' || host === '[::1]' || host === '::1';
	} catch {
		return false;
	}
}

function assertLoopbackBase(raw) {
	const base = normalizeNativeBase(raw);
	if (!isLoopbackURL(base)) {
		throw new Error('Ollama Local only allows loopback endpoints (127.0.0.1 / localhost)');
	}
	return base;
}

function isValidModelName(name) {
	const trimmed = String(name || '').trim();
	if (!trimmed || trimmed.length > 200) return false;
	if (trimmed.includes('://') || trimmed.includes('/') || trimmed.includes('\\')) return false;
	return OLLAMA_MODEL_NAME_RE.test(trimmed);
}

function resolvePullName({ catalogId, modelName }) {
	if (catalogId) {
		const entry = OLLAMA_CATALOG.find((item) => item.id === String(catalogId).trim());
		if (!entry) throw new Error(`Unknown Ollama catalog model: ${catalogId}`);
		return entry.pullName;
	}
	const name = String(modelName || '').trim();
	if (!isValidModelName(name)) {
		throw new Error('Invalid Ollama model name');
	}
	return name;
}

/**
 * @param {{
 *   fetchImpl?: typeof fetch,
 *   sendProgress?: (payload: object) => void
 * }} [deps]
 */
function createOllamaService(deps = {}) {
	const fetchImpl = deps.fetchImpl || globalThis.fetch.bind(globalThis);
	const sendProgress = typeof deps.sendProgress === 'function' ? deps.sendProgress : () => {};

	/** @type {{ controller: AbortController, model: string } | null} */
	let activePull = null;

	async function checkHealth(baseURL) {
		const base = assertLoopbackBase(baseURL);
		const controller = new AbortController();
		const timer = setTimeout(() => controller.abort(), 4000);
		try {
			const res = await fetchImpl(`${base}/api/version`, { signal: controller.signal });
			if (!res.ok) {
				return {
					ok: false,
					state: 'unreachable',
					baseURL: base,
					error: `Ollama returned ${res.status}`
				};
			}
			const body = await res.json().catch(() => ({}));
			return {
				ok: true,
				state: 'healthy',
				baseURL: base,
				version: String(body?.version || '')
			};
		} catch (error) {
			return {
				ok: false,
				state: 'missing',
				baseURL: base,
				error: error instanceof Error ? error.message : String(error)
			};
		} finally {
			clearTimeout(timer);
		}
	}

	async function listModels(baseURL) {
		const base = assertLoopbackBase(baseURL);
		const res = await fetchImpl(`${base}/api/tags`, {
			signal: AbortSignal.timeout(10000)
		});
		if (!res.ok) {
			const text = await res.text().catch(() => '');
			throw new Error(`Failed to list Ollama models: ${res.status} ${text.slice(0, 200)}`);
		}
		const body = await res.json();
		const models = Array.isArray(body?.models) ? body.models : [];
		return {
			baseURL: base,
			models: models
				.map((model) => ({
					name: String(model?.name || '').trim(),
					size: typeof model?.size === 'number' ? model.size : undefined,
					digest: String(model?.digest || '').trim() || undefined,
					modifiedAt: String(model?.modified_at || '').trim() || undefined
				}))
				.filter((model) => model.name)
		};
	}

	async function getDiagnostics(baseURL) {
		const health = await checkHealth(baseURL);
		let models = [];
		if (health.ok) {
			try {
				const listed = await listModels(health.baseURL);
				models = listed.models;
			} catch {
				// keep health-only diagnostics
			}
		}
		return {
			...health,
			models,
			pullActive: Boolean(activePull),
			pullModel: activePull?.model || null
		};
	}

	async function pullModel({ baseURL, catalogId, modelName }) {
		if (activePull) {
			throw new Error('Another Ollama model pull is already in progress');
		}
		const base = assertLoopbackBase(baseURL);
		const model = resolvePullName({ catalogId, modelName });
		const controller = new AbortController();
		activePull = { controller, model };
		const timer = setTimeout(() => controller.abort(), PULL_TIMEOUT_MS);

		try {
			const health = await checkHealth(base);
			if (!health.ok) {
				throw new Error(health.error || 'Ollama is not running');
			}

			const res = await fetchImpl(`${base}/api/pull`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: model, stream: true }),
				signal: controller.signal
			});
			if (!res.ok || !res.body) {
				const text = await res.text().catch(() => '');
				throw new Error(`Ollama pull failed: ${res.status} ${text.slice(0, 200)}`);
			}

			const reader = res.body.getReader();
			const decoder = new TextDecoder();
			let buffer = '';
			let lastStatus = '';
			let completed = false;

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;
				buffer += decoder.decode(value, { stream: true });
				if (buffer.length > MAX_LINE_BYTES * 8) {
					throw new Error('Ollama pull stream exceeded size limit');
				}
				const lines = buffer.split('\n');
				buffer = lines.pop() || '';
				for (const line of lines) {
					const trimmed = line.trim();
					if (!trimmed) continue;
					if (trimmed.length > MAX_LINE_BYTES) {
						throw new Error('Ollama pull progress line too large');
					}
					let event;
					try {
						event = JSON.parse(trimmed);
					} catch {
						continue;
					}
					if (event?.error) {
						throw new Error(String(event.error));
					}
					lastStatus = String(event?.status || lastStatus);
					const total = typeof event?.total === 'number' ? event.total : undefined;
					const completedBytes =
						typeof event?.completed === 'number' ? event.completed : undefined;
					const percent =
						total && completedBytes != null && total > 0
							? Math.min(100, Math.round((completedBytes / total) * 100))
							: undefined;
					sendProgress({
						model,
						status: lastStatus,
						total,
						completed: completedBytes,
						percent,
						done: Boolean(event?.status === 'success')
					});
					if (event?.status === 'success') {
						completed = true;
					}
				}
			}

			if (!completed && lastStatus && lastStatus !== 'success') {
				// Some Ollama versions end the stream without a final success line after digest.
				completed = true;
			}

			const listed = await listModels(base);
			return {
				ok: true,
				model,
				models: listed.models
			};
		} catch (error) {
			if (controller.signal.aborted) {
				const err = new Error('Ollama pull cancelled');
				err.code = 'CANCELLED';
				throw err;
			}
			throw error;
		} finally {
			clearTimeout(timer);
			activePull = null;
		}
	}

	function cancelPull() {
		if (!activePull) {
			return { ok: true, cancelled: false };
		}
		activePull.controller.abort();
		return { ok: true, cancelled: true, model: activePull.model };
	}

	return {
		checkHealth,
		listModels,
		getDiagnostics,
		pullModel,
		cancelPull,
		normalizeNativeBase,
		isValidModelName,
		resolvePullName,
		OLLAMA_CATALOG
	};
}

export {
	createOllamaService,
	normalizeNativeBase,
	isValidModelName,
	resolvePullName,
	OLLAMA_CATALOG,
	DEFAULT_NATIVE_BASE
};
