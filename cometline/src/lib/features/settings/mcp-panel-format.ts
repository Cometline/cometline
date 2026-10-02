import type { MCPServerConfig, MCPTransport } from '#lib/cometmind-settings.js';
import type { McpServerStatus, McpToolInfo } from '#lib/client/cometmind.js';

export const MCP_REFRESH_TIMEOUT_MS = 8_000;
export const MCP_RELOADING_POLL_MS = 1_500;
export const MCP_RELOADING_STATUS_MESSAGE = 'CometMind is reloading MCP servers…';

export type KnownMcpTool = { name: string; description: string };

export const transportOptions: { value: MCPTransport; label: string; hint: string }[] = [
	{
		value: 'stdio',
		label: 'Local command',
		hint: 'Run npx, node, uvx, or another CLI on this machine.'
	},
	{
		value: 'http',
		label: 'Remote URL (HTTP)',
		hint: 'Connect to a hosted MCP server over HTTP.'
	}
];

export function transportHint(value: MCPTransport): string {
	return transportOptions.find((option) => option.value === value)?.hint ?? '';
}

export function formatEnv(values: Record<string, string> | undefined): string {
	if (!values) return '';
	return Object.entries(values)
		.map(([key, value]) => `${key}=${value}`)
		.join('\n');
}

export function parseEnv(raw: string): Record<string, string> {
	const out: Record<string, string> = {};
	for (const line of raw.split('\n')) {
		const trimmed = line.trim();
		if (!trimmed) continue;
		const idx = trimmed.indexOf('=');
		if (idx <= 0) continue;
		const key = trimmed.slice(0, idx).trim();
		const value = trimmed.slice(idx + 1).trim();
		if (key) out[key] = value;
	}
	return out;
}

export function parseArgs(raw: string): string[] {
	return raw
		.split(/\s+/)
		.map((part) => part.trim())
		.filter(Boolean);
}

export function parseServerTextFields(texts: { args: string; env: string; headers: string }) {
	return {
		args: parseArgs(texts.args),
		env: parseEnv(texts.env),
		headers: parseEnv(texts.headers)
	};
}

/**
 * Overlay persisted server configs onto the editing draft, skipping the
 * server being edited (`keepServerId`) and appending persisted servers the
 * draft does not have yet.
 */
export function mergePersistedServers(
	draft: MCPServerConfig[],
	persisted: MCPServerConfig[],
	keepServerId: string | null
): MCPServerConfig[] {
	const persistedById = new Map(persisted.map((server) => [server.id, server]));
	const nextServers = draft.map((server) => {
		if (server.id === keepServerId) return server;
		const saved = persistedById.get(server.id);
		return saved ? { ...server, ...saved } : server;
	});
	for (const saved of persisted) {
		if (!nextServers.some((server) => server.id === saved.id)) {
			nextServers.push(saved);
		}
	}
	return nextServers;
}

export function newServerId(servers: MCPServerConfig[]): string {
	const candidate = `server-${Date.now()}`;
	return servers.some((server) => server.id === candidate) ? `${candidate}-1` : candidate;
}

export function connectionSummary(server: MCPServerConfig): string {
	if (server.transport === 'stdio') {
		const command = String(server.command ?? '').trim();
		const args = (server.args ?? []).join(' ');
		return [command, args].filter(Boolean).join(' ') || 'No command configured';
	}
	return String(server.url ?? '').trim() || 'No URL configured';
}

export function statusLabel(
	status: McpServerStatus | undefined,
	server: MCPServerConfig,
	mcpEnabled: boolean
): string {
	if (!mcpEnabled) return 'Off';
	if (!server.enabled) return 'Disabled';
	if (!status) return 'Unknown';
	if (
		status.status === 'error' &&
		(status.error_code === 'needs_auth' ||
			status.error_code === 'auth_expired' ||
			status.error_code === 'unauthorized')
	)
		return 'Needs sign-in';
	if (status.status === 'connecting') return 'Connecting';
	return status.status;
}

export function statusClass(
	status: McpServerStatus | undefined,
	server: MCPServerConfig,
	mcpEnabled: boolean
): string {
	const value = statusLabel(status, server, mcpEnabled);
	if (value === 'connected') return 'connected';
	if (value === 'Needs sign-in') return 'pending';
	if (value === 'error' || value === 'disconnected') return 'error';
	if (value === 'reloading' || value === 'Connecting' || value === 'connecting') return 'pending';
	if (value === 'Disabled' || value === 'Off') return 'idle';
	return 'idle';
}

export function displayError(status: McpServerStatus | undefined, expanded = false): string {
	if (!status) return '';
	const hint = status.error_hint ?? '';
	const raw = status.last_error ?? '';
	if (!expanded) return hint || raw;
	if (hint && raw && hint !== raw) return `${hint}\n${raw}`;
	return hint || raw;
}

export function hasPendingServer(statuses: McpServerStatus[]): boolean {
	return statuses.some(
		(server) => server.status === 'reloading' || server.status === 'connecting'
	);
}

/**
 * Merge remembered tools with freshly discovered tools and extra tool names
 * (e.g. a saved allow-list), sorted by name. Discovered descriptions win over
 * remembered ones; extra names never overwrite an existing entry.
 */
export function mergeKnownTools(
	remembered: KnownMcpTool[],
	discovered: McpToolInfo[],
	extraNames: string[]
): KnownMcpTool[] {
	const byName = new Map<string, string>();
	for (const existing of remembered) byName.set(existing.name, existing.description);
	for (const tool of discovered) {
		const name = tool.tool_name?.trim();
		if (!name) continue;
		byName.set(name, tool.description || byName.get(name) || '');
	}
	for (const name of extraNames) {
		const clean = name.trim();
		if (clean && !byName.has(clean)) byName.set(clean, '');
	}
	return [...byName.entries()]
		.map(([name, description]) => ({ name, description }))
		.sort((a, b) => a.name.localeCompare(b.name));
}

export function sameToolNames(prev: KnownMcpTool[], next: KnownMcpTool[]): boolean {
	return prev.length === next.length && !prev.some((t, i) => t.name !== next[i]?.name);
}

/** A tool is allowed when the allow-list is empty (expose all) or contains it. */
export function isToolAllowed(server: MCPServerConfig, toolName: string): boolean {
	const allow = server.allowedTools ?? [];
	return allow.length === 0 || allow.includes(toolName);
}

/**
 * Toggle a single tool on/off. An empty allow-list means "all allowed", so the
 * first time a user turns one tool off we materialize the full known list minus
 * that tool. If every known tool ends up enabled again, collapse back to an
 * empty list (the "expose everything" default, including tools discovered later).
 */
export function toggledAllowedTools(
	known: string[],
	current: string[],
	toolName: string
): string[] {
	const currentlyAllowed = current.length === 0 ? new Set(known) : new Set(current);

	if (currentlyAllowed.has(toolName)) {
		currentlyAllowed.delete(toolName);
	} else {
		currentlyAllowed.add(toolName);
	}

	const allEnabled = known.length > 0 && known.every((name) => currentlyAllowed.has(name));
	return allEnabled ? [] : known.filter((name) => currentlyAllowed.has(name));
}

export function isMcpStatusError(message: string): boolean {
	return message.toLowerCase().includes('fail') || message.toLowerCase().includes('invalid');
}

export async function withTimeout<T>(
	promise: Promise<T>,
	label: string,
	timeoutMs = MCP_REFRESH_TIMEOUT_MS
): Promise<T> {
	let timeoutId: ReturnType<typeof setTimeout> | undefined;
	const timeout = new Promise<never>((_, reject) => {
		timeoutId = setTimeout(
			() => reject(new Error(`${label} timed out after ${timeoutMs / 1000}s`)),
			timeoutMs
		);
	});
	try {
		return await Promise.race([promise, timeout]);
	} finally {
		if (timeoutId) clearTimeout(timeoutId);
	}
}
