import { OLLAMA_DEFAULT_NATIVE_BASE } from './catalog';

/** Canonical storage form: native daemon base without `/v1`. */
export function normalizeOllamaNativeBase(url: string | undefined): string {
	let base = String(url ?? '')
		.trim()
		.replace(/\/+$/, '');
	if (!base) return OLLAMA_DEFAULT_NATIVE_BASE;
	if (base.toLowerCase().endsWith('/v1')) {
		base = base.slice(0, -3).replace(/\/+$/, '');
	}
	return base || OLLAMA_DEFAULT_NATIVE_BASE;
}

/** OpenAI-compatible chat completions base (`…/v1`). */
export function ollamaChatBaseURL(nativeBase: string | undefined): string {
	return `${normalizeOllamaNativeBase(nativeBase)}/v1`;
}

export function isLoopbackOllamaURL(url: string | undefined): boolean {
	try {
		const parsed = new URL(normalizeOllamaNativeBase(url));
		const host = parsed.hostname.toLowerCase();
		return host === '127.0.0.1' || host === 'localhost' || host === '[::1]' || host === '::1';
	} catch {
		return false;
	}
}
