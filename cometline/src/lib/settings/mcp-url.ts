import type { MCPServerConfig } from '$lib/cometmind-settings';

/**
 * Normalization helpers for HTTP/SSE MCP server connection details.
 *
 * Some MCP servers (e.g. Typefully) authenticate via an API key in the URL
 * *query string* (`?TYPEFULLY_API_KEY=...`) or an `Authorization: Bearer`
 * header — not via an arbitrary custom header named after the key. The
 * settings UI historically encouraged users to put every credential into a
 * custom header, which silently breaks query-param-auth servers (the server
 * returns HTTP 400 "Missing API key" → `sending "initialize": Bad Request`).
 *
 * These helpers keep the URL field as the single source of truth and let a
 * misplaced API-key header be folded back into the URL query string.
 */

/** Matches header names that look like an API key / token credential. */
const API_KEY_HEADER = /^(?:[a-z0-9]+_)*(?:api[_-]?key|api[_-]?token|access[_-]?token|token|key)$/i;

/** A header that already carries the credential the canonical way. */
function isBearerHeader(name: string): boolean {
	return name.trim().toLowerCase() === 'authorization';
}

export function looksLikeApiKeyHeader(name: string): boolean {
	const trimmed = name.trim();
	if (!trimmed || isBearerHeader(trimmed)) return false;
	return API_KEY_HEADER.test(trimmed);
}

/** Returns the lower-cased set of query parameter names present in a URL. */
function queryParamNames(url: string): Set<string> {
	const out = new Set<string>();
	const qIndex = url.indexOf('?');
	if (qIndex === -1) return out;
	const query = url.slice(qIndex + 1);
	for (const pair of query.split('&')) {
		if (!pair) continue;
		const eq = pair.indexOf('=');
		const rawName = eq === -1 ? pair : pair.slice(0, eq);
		const name = decodeURIComponent(rawName).trim().toLowerCase();
		if (name) out.add(name);
	}
	return out;
}

/**
 * Append a single `name=value` query parameter to a URL string, preserving any
 * existing query string and fragment. Does nothing if a parameter with the
 * same name (case-insensitive) is already present.
 */
export function appendQueryParam(url: string, name: string, value: string): string {
	const trimmedUrl = url.trim();
	const cleanName = name.trim();
	const cleanValue = value.trim();
	if (!trimmedUrl || !cleanName || !cleanValue) return trimmedUrl;
	if (queryParamNames(trimmedUrl).has(cleanName.toLowerCase())) return trimmedUrl;

	const hashIndex = trimmedUrl.indexOf('#');
	const fragment = hashIndex === -1 ? '' : trimmedUrl.slice(hashIndex);
	const base = hashIndex === -1 ? trimmedUrl : trimmedUrl.slice(0, hashIndex);

	const separator = base.includes('?') ? '&' : '?';
	const encoded = `${encodeURIComponent(cleanName)}=${encodeURIComponent(cleanValue)}`;
	return `${base}${separator}${encoded}${fragment}`;
}

export interface NormalizedConnection {
	url: string;
	headers: Record<string, string>;
	/** Header names that were folded into the URL query string. */
	movedToQuery: string[];
}

/**
 * Fold misplaced API-key headers into the URL query string for HTTP/SSE
 * servers. A header is moved when:
 *   - its name matches the URL's existing query parameter name (the user
 *     clearly intends query-param auth), or
 *   - it looks like an API key/token AND the URL has no query string at all
 *     (so we are not overriding an existing, working configuration).
 *
 * `Authorization` and non-credential headers are always left untouched.
 */
export function normalizeHttpConnection(
	url: string,
	headers: Record<string, string> | undefined
): NormalizedConnection {
	const trimmedUrl = (url ?? '').trim();
	const entries = Object.entries(headers ?? {});
	if (!trimmedUrl || entries.length === 0) {
		return { url: trimmedUrl, headers: { ...(headers ?? {}) }, movedToQuery: [] };
	}

	const existingParams = queryParamNames(trimmedUrl);
	const hasQuery = existingParams.size > 0;

	let nextUrl = trimmedUrl;
	const nextHeaders: Record<string, string> = {};
	const movedToQuery: string[] = [];

	for (const [name, value] of entries) {
		const cleanName = name.trim();
		const cleanValue = String(value ?? '').trim();
		if (!cleanName || !cleanValue) continue;

		const matchesExistingParam = existingParams.has(cleanName.toLowerCase());

		// The credential is already supplied via the URL query string; drop the
		// redundant (and ignored) custom header so it cannot mask the real auth.
		if (matchesExistingParam) {
			movedToQuery.push(cleanName);
			continue;
		}

		const foldable = !hasQuery && looksLikeApiKeyHeader(cleanName);
		if (foldable) {
			const folded = appendQueryParam(nextUrl, cleanName, cleanValue);
			if (folded !== nextUrl) {
				nextUrl = folded;
				movedToQuery.push(cleanName);
				continue;
			}
		}
		nextHeaders[cleanName] = cleanValue;
	}

	return { url: nextUrl, headers: nextHeaders, movedToQuery };
}

/**
 * Apply {@link normalizeHttpConnection} to an MCP server config when it uses an
 * HTTP/SSE transport. Returns the same object reference when nothing changed so
 * callers can cheaply detect whether an update is needed.
 */
export function normalizeServerConnection(server: MCPServerConfig): MCPServerConfig {
	if (server.transport !== 'http' && server.transport !== 'sse') return server;
	const { url, headers, movedToQuery } = normalizeHttpConnection(
		server.url ?? '',
		server.headers
	);
	if (movedToQuery.length === 0 && url === (server.url ?? '').trim()) return server;
	return { ...server, url, headers };
}
