import { OLLAMA_DEFAULT_NATIVE_BASE, OLLAMA_DOWNLOAD_URL } from './catalog';

export type OllamaHealthResult = {
	ok: boolean;
	state: 'healthy' | 'missing' | 'unreachable' | string;
	baseURL: string;
	version?: string;
	error?: string;
};

export type OllamaInstalledModel = {
	name: string;
	size?: number;
	digest?: string;
	modifiedAt?: string;
};

export type OllamaPullProgress = {
	model: string;
	status: string;
	total?: number;
	completed?: number;
	percent?: number;
	done?: boolean;
};

function api() {
	return typeof window !== 'undefined' ? window.electronAPI : undefined;
}

export async function checkOllamaHealth(
	baseURL = OLLAMA_DEFAULT_NATIVE_BASE
): Promise<OllamaHealthResult> {
	const fn = api()?.checkOllamaHealth;
	if (!fn) {
		return {
			ok: false,
			state: 'missing',
			baseURL,
			error: 'Ollama health checks require the desktop app'
		};
	}
	return fn(baseURL);
}

export async function listOllamaModels(baseURL = OLLAMA_DEFAULT_NATIVE_BASE) {
	const fn = api()?.listOllamaModels;
	if (!fn) throw new Error('Ollama model listing requires the desktop app');
	return fn(baseURL);
}

export async function pullOllamaModel(payload: {
	baseURL?: string;
	catalogId?: string;
	modelName?: string;
}) {
	const fn = api()?.pullOllamaModel;
	if (!fn) throw new Error('Ollama pull requires the desktop app');
	return fn({
		baseURL: payload.baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
		catalogId: payload.catalogId,
		modelName: payload.modelName
	});
}

export async function cancelOllamaPull() {
	const fn = api()?.cancelOllamaPull;
	if (!fn) return { ok: false, cancelled: false };
	return fn();
}

export function onOllamaPullProgress(callback: (payload: OllamaPullProgress) => void) {
	const fn = api()?.onOllamaPullProgress;
	if (!fn) return () => {};
	return fn(callback);
}

export async function openOllamaDownloadPage() {
	const open = api()?.openExternal;
	if (!open) {
		window.open(OLLAMA_DOWNLOAD_URL, '_blank', 'noopener,noreferrer');
		return;
	}
	await open(OLLAMA_DOWNLOAD_URL);
}

export function formatBytes(bytes: number | undefined): string {
	if (bytes == null || !Number.isFinite(bytes) || bytes <= 0) return '';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let value = bytes;
	let unit = 0;
	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024;
		unit += 1;
	}
	return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}
