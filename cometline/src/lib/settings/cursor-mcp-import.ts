import type { MCPServerConfig, MCPTransport } from '$lib/cometmind-settings';

type CursorMcpEntry = {
	command?: unknown;
	args?: unknown;
	env?: unknown;
	url?: unknown;
	headers?: unknown;
	type?: unknown;
	disabled?: unknown;
};

function cleanStringList(value: unknown): string[] {
	if (!Array.isArray(value)) return [];
	return value.map((part) => String(part ?? '').trim()).filter(Boolean);
}

function cleanStringMap(value: unknown): Record<string, string> {
	if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
	const out: Record<string, string> = {};
	for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
		const k = String(key ?? '').trim();
		const v = String(raw ?? '').trim();
		if (k && v) out[k] = v;
	}
	return out;
}

function slugifyMCPId(name: string, existing: Set<string>): string {
	const base =
		name
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '') || 'server';
	let candidate = base;
	let n = 2;
	while (existing.has(candidate)) {
		candidate = `${base}-${n++}`;
	}
	existing.add(candidate);
	return candidate;
}

function inferTransport(entry: CursorMcpEntry): MCPTransport | null {
	const url = String(entry.url ?? '').trim();
	if (!url) return 'stdio';

	const type = String(entry.type ?? '')
		.trim()
		.toLowerCase();
	if (type === 'sse') return 'sse';
	if (type === 'http' || type === 'streamablehttp' || type === 'streamable-http') return 'http';
	return 'http';
}

function parseCursorMcpEntry(
	key: string,
	entry: CursorMcpEntry,
	ids: Set<string>
): MCPServerConfig | null {
	const transport = inferTransport(entry);
	if (!transport) return null;

	if (transport === 'stdio') {
		const command = String(entry.command ?? '').trim();
		if (!command) return null;
		const id = slugifyMCPId(key, ids);
		return {
			id,
			name: key,
			enabled: entry.disabled !== true,
			transport: 'stdio',
			command,
			args: cleanStringList(entry.args),
			env: cleanStringMap(entry.env),
			url: '',
			headers: {}
		};
	}

	const url = String(entry.url ?? '').trim();
	if (!url) return null;
	const id = slugifyMCPId(key, ids);
	return {
		id,
		name: key,
		enabled: entry.disabled !== true,
		transport,
		command: '',
		args: [],
		env: {},
		url,
		headers: cleanStringMap(entry.headers)
	};
}

/** Parse Cursor-style `{ "mcpServers": { ... } }` into CometMind MCP server configs. */
export function parseCursorMcpJson(
	input: unknown,
	existingIds: Iterable<string> = []
): MCPServerConfig[] {
	if (!input || typeof input !== 'object' || Array.isArray(input)) return [];
	const mcpServers = (input as { mcpServers?: unknown }).mcpServers;
	if (!mcpServers || typeof mcpServers !== 'object' || Array.isArray(mcpServers)) return [];

	const ids = new Set(existingIds);
	const servers: MCPServerConfig[] = [];
	for (const [key, value] of Object.entries(mcpServers as Record<string, unknown>)) {
		if (!value || typeof value !== 'object' || Array.isArray(value)) continue;
		const parsed = parseCursorMcpEntry(key, value as CursorMcpEntry, ids);
		if (parsed) servers.push(parsed);
	}
	return servers;
}

/** Append imported servers, assigning unique IDs when they collide with existing entries. */
export function mergeImportedMcpServers(
	existing: MCPServerConfig[],
	imported: MCPServerConfig[]
): MCPServerConfig[] {
	if (imported.length === 0) return existing;
	const ids = new Set(existing.map((server) => server.id));
	const merged = [...existing];
	for (const server of imported) {
		let id = server.id;
		if (ids.has(id)) {
			const slugIds = new Set(ids);
			id = slugifyMCPId(server.id, slugIds);
		} else {
			ids.add(id);
		}
		merged.push({ ...server, id });
	}
	return merged;
}
