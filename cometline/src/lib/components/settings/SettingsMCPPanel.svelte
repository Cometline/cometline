<script lang="ts">
	import SettingsToggle from './SettingsToggle.svelte';
	import SettingsButton from './SettingsButton.svelte';
	import SettingsField from './SettingsField.svelte';
	import {
		type CometMindMCPSettings,
		type MCPServerConfig,
		type MCPTransport
	} from '$lib/cometmind-settings';
	import { mergeImportedMcpServers, parseCursorMcpJson } from '$lib/settings/cursor-mcp-import';
	import { normalizeServerConnection } from '$lib/settings/mcp-url';
	import {
		apiErrorMessage,
		listMcpServers,
		listMcpTools,
		reconnectMcpServer,
		startMcpOAuth,
		testMcpServer,
		type McpServerStatus,
		type McpToolInfo
	} from '$lib/client/cometmind';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { ChevronDown, ChevronRight, Download, Plus, RefreshCw, Trash2 } from '@lucide/svelte';
	import { onMount } from 'svelte';

	let {
		mcp = $bindable(),
		onPersistBeforeRuntimeAction
	}: {
		mcp: CometMindMCPSettings;
		onPersistBeforeRuntimeAction?: (overrides?: { mcp: CometMindMCPSettings }) => Promise<void>;
	} = $props();

	const MCP_REFRESH_TIMEOUT_MS = 8_000;
	const MCP_RELOADING_POLL_MS = 1_500;
	const MCP_RELOADING_STATUS_MESSAGE = 'CometMind is reloading MCP servers…';

	const transportOptions: { value: MCPTransport; label: string; hint: string }[] = [
		{
			value: 'stdio',
			label: 'Local command',
			hint: 'Run npx, node, uvx, or another CLI on this machine.'
		},
		{
			value: 'http',
			label: 'Remote URL (HTTP)',
			hint: 'Connect to a hosted MCP server over HTTP.'
		},
		{ value: 'sse', label: 'Remote URL (SSE)', hint: 'Legacy server-sent events transport.' }
	];

	let serverStatuses = $state<McpServerStatus[]>([]);
	let toolPreview = $state<McpToolInfo[]>([]);
	let mcpBusy = $state(false);
	let rowBusy = $state<Record<string, 'reconnect' | 'oauth' | 'test'>>({});
	let autoRefreshInFlight = $state(false);
	let mcpStatus = $state('');
	let envTexts = $state<Record<string, string>>({});
	let headerTexts = $state<Record<string, string>>({});
	let argsTexts = $state<Record<string, string>>({});
	let expandedServerId = $state<string | null>(null);
	/**
	 * Tools we have ever seen for a server during this Settings session, keyed by
	 * server id. Seeded from discovered tools (toolPreview) and each server's
	 * saved allow-list so toggles stay visible even after a tool is disallowed
	 * (the backend stops reporting disallowed tools). UI-only; not persisted.
	 */
	let knownToolsByServer = $state<Record<string, { name: string; description: string }[]>>({});

	function setTextField(map: Record<string, string>, id: string, value: string) {
		return { ...map, [id]: value };
	}

	function syncTextFieldsFromSettings() {
		const nextEnv: Record<string, string> = {};
		const nextHeaders: Record<string, string> = {};
		const nextArgs: Record<string, string> = {};
		for (const server of mcp.servers ?? []) {
			nextEnv[server.id] = formatEnv(server.env);
			nextHeaders[server.id] = formatEnv(server.headers);
			nextArgs[server.id] = (server.args ?? []).join(' ');
		}
		envTexts = nextEnv;
		headerTexts = nextHeaders;
		argsTexts = nextArgs;
	}

	// Only re-seed local text mirrors when the server *id set* changes (add/remove/import).
	// syncServerLists() replaces the servers array on every env/args/header keystroke so
	// the effect re-runs often; guard so we do not clobber in-progress textarea input
	// (e.g. "FOO" before "=" was typed).
	let lastServerIdKey = '';

	$effect(() => {
		const key = (mcp.servers ?? []).map((server) => server.id).join('\0');
		if (key === lastServerIdKey) return;
		lastServerIdKey = key;
		syncTextFieldsFromSettings();
	});

	onMount(() => {
		void refreshMcpRuntime();
	});

	$effect(() => {
		const hasPendingServer = serverStatuses.some(
			(server) => server.status === 'reloading' || server.status === 'connecting'
		);
		if (!hasPendingServer || mcpBusy || autoRefreshInFlight) return;

		const timeoutId = setTimeout(() => {
			void refreshMcpRuntime({ silent: true });
		}, MCP_RELOADING_POLL_MS);

		return () => clearTimeout(timeoutId);
	});

	function formatEnv(values: Record<string, string> | undefined): string {
		if (!values) return '';
		return Object.entries(values)
			.map(([key, value]) => `${key}=${value}`)
			.join('\n');
	}

	function parseEnv(raw: string): Record<string, string> {
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

	function updateMcp(patch: Partial<CometMindMCPSettings>) {
		mcp = { ...mcp, ...patch };
	}

	function newServerId(): string {
		const servers = mcp.servers ?? [];
		const candidate = `server-${Date.now()}`;
		return servers.some((server) => server.id === candidate) ? `${candidate}-1` : candidate;
	}

	function updateServer(serverId: string, patch: Partial<MCPServerConfig>) {
		updateMcp({
			servers: mcp.servers.map((server) =>
				server.id === serverId ? { ...server, ...patch } : server
			)
		});
	}

	function addServer() {
		const servers = mcp.servers ?? [];
		const id = newServerId();
		const server: MCPServerConfig = {
			id,
			name: `MCP Server ${servers.length + 1}`,
			enabled: true,
			transport: 'stdio',
			command: '',
			args: [],
			env: {},
			url: '',
			headers: {}
		};
		updateMcp({ enabled: true, servers: [...servers, server] });
		expandedServerId = id;
	}

	async function importFromCursor() {
		if (!window.electronAPI?.readCursorMcpConfig) {
			mcpStatus = 'Import from Cursor is only available in the desktop app.';
			return;
		}
		mcpBusy = true;
		mcpStatus = '';
		try {
			const result = await window.electronAPI.readCursorMcpConfig();
			if (!result.ok) {
				mcpStatus = result.error;
				return;
			}
			const existing = mcp.servers ?? [];
			const imported = parseCursorMcpJson(
				result.config,
				existing.map((server) => server.id)
			);
			if (imported.length === 0) {
				mcpStatus = 'No MCP servers found in Cursor config.';
				return;
			}
			updateMcp({
				enabled: true,
				servers: mergeImportedMcpServers(existing, imported)
			});
			expandedServerId = imported[0]?.id ?? expandedServerId;
			mcpStatus = `Imported ${imported.length} server(s) from Cursor. Save settings to apply.`;
		} catch (err) {
			mcpStatus = err instanceof Error ? err.message : 'Failed to import from Cursor';
		} finally {
			mcpBusy = false;
		}
	}

	function removeServer(serverId: string) {
		updateMcp({ servers: mcp.servers.filter((server) => server.id !== serverId) });
		if (expandedServerId === serverId) expandedServerId = null;
	}

	function toggleExpanded(serverId: string) {
		expandedServerId = expandedServerId === serverId ? null : serverId;
	}

	function statusFor(serverId: string): McpServerStatus | undefined {
		return serverStatuses.find((item) => item.id === serverId);
	}

	function toolsForServer(serverId: string): McpToolInfo[] {
		return (toolPreview ?? []).filter((tool) => tool.server_id === serverId);
	}

	/**
	 * Pure reader: merge the remembered tools with the server's saved allow-list
	 * and return the sorted list to render as toggles. Remembering happens in
	 * {@link rememberDiscoveredTools}; this never mutates state so it is safe to
	 * call from the template.
	 */
	function knownToolsFor(server: MCPServerConfig): { name: string; description: string }[] {
		const byName = new Map<string, string>();
		for (const existing of knownToolsByServer[server.id] ?? []) {
			byName.set(existing.name, existing.description);
		}
		for (const name of server.allowedTools ?? []) {
			const clean = name.trim();
			if (clean && !byName.has(clean)) byName.set(clean, '');
		}
		return [...byName.entries()]
			.map(([name, description]) => ({ name, description }))
			.sort((a, b) => a.name.localeCompare(b.name));
	}

	/**
	 * Fold the currently discovered tools (toolPreview) into the session-scoped
	 * known-tools memory. Called after the tool list refreshes. Disallowed tools
	 * drop out of the backend's discovered list, so once seen we keep them.
	 */
	function rememberDiscoveredTools() {
		let changed = false;
		const next = { ...knownToolsByServer };
		for (const server of mcp.servers ?? []) {
			const byName = new Map<string, string>();
			for (const existing of next[server.id] ?? [])
				byName.set(existing.name, existing.description);
			for (const tool of toolsForServer(server.id)) {
				const name = tool.tool_name?.trim();
				if (!name) continue;
				byName.set(name, tool.description || byName.get(name) || '');
			}
			for (const name of server.allowedTools ?? []) {
				const clean = name.trim();
				if (clean && !byName.has(clean)) byName.set(clean, '');
			}
			const merged = [...byName.entries()]
				.map(([name, description]) => ({ name, description }))
				.sort((a, b) => a.name.localeCompare(b.name));
			const prev = next[server.id] ?? [];
			if (prev.length !== merged.length || prev.some((t, i) => t.name !== merged[i]?.name)) {
				next[server.id] = merged;
				changed = true;
			}
		}
		if (changed) knownToolsByServer = next;
	}

	/** A tool is allowed when the allow-list is empty (expose all) or contains it. */
	function isToolAllowed(server: MCPServerConfig, toolName: string): boolean {
		const allow = server.allowedTools ?? [];
		return allow.length === 0 || allow.includes(toolName);
	}

	/**
	 * Toggle a single tool on/off. An empty allow-list means "all allowed", so the
	 * first time a user turns one tool off we materialize the full known list minus
	 * that tool. If every known tool ends up enabled again, collapse back to an
	 * empty list (the "expose everything" default, including tools discovered later).
	 */
	function toggleTool(serverId: string, toolName: string) {
		const server = mcp.servers.find((item) => item.id === serverId);
		if (!server) return;
		const known = (knownToolsByServer[serverId] ?? []).map((tool) => tool.name);
		const current = server.allowedTools ?? [];
		const currentlyAllowed = current.length === 0 ? new Set(known) : new Set(current);

		if (currentlyAllowed.has(toolName)) {
			currentlyAllowed.delete(toolName);
		} else {
			currentlyAllowed.add(toolName);
		}

		const allEnabled = known.length > 0 && known.every((name) => currentlyAllowed.has(name));
		const next = allEnabled ? [] : known.filter((name) => currentlyAllowed.has(name));
		updateServer(serverId, { allowedTools: next });
	}

	function statusLabel(status: McpServerStatus | undefined, server: MCPServerConfig): string {
		if (!mcp.enabled) return 'Off';
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

	function statusClass(status: McpServerStatus | undefined, server: MCPServerConfig): string {
		const value = statusLabel(status, server);
		if (value === 'connected') return 'connected';
		if (value === 'Needs sign-in') return 'pending';
		if (value === 'error' || value === 'disconnected') return 'error';
		if (value === 'reloading' || value === 'Connecting' || value === 'connecting') return 'pending';
		if (value === 'Disabled' || value === 'Off') return 'idle';
		return 'idle';
	}

	function displayError(status: McpServerStatus | undefined, expanded = false): string {
		if (!status) return '';
		const hint = status.error_hint ?? '';
		const raw = status.last_error ?? '';
		if (!expanded) return hint || raw;
		if (hint && raw && hint !== raw) return `${hint}\n${raw}`;
		return hint || raw;
	}

	function setRowBusy(serverId: string, op: 'reconnect' | 'oauth' | 'test' | null) {
		if (op) {
			rowBusy = { ...rowBusy, [serverId]: op };
			return;
		}
		const next = { ...rowBusy };
		delete next[serverId];
		rowBusy = next;
	}

	function isRowBusy(serverId: string): boolean {
		return Boolean(rowBusy[serverId]);
	}

	function connectionSummary(server: MCPServerConfig): string {
		if (server.transport === 'stdio') {
			const command = String(server.command ?? '').trim();
			const args = (server.args ?? []).join(' ');
			return [command, args].filter(Boolean).join(' ') || 'No command configured';
		}
		return String(server.url ?? '').trim() || 'No URL configured';
	}

	function transportHint(value: MCPTransport): string {
		return transportOptions.find((option) => option.value === value)?.hint ?? '';
	}

	async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
		let timeoutId: ReturnType<typeof setTimeout> | undefined;
		const timeout = new Promise<never>((_, reject) => {
			timeoutId = setTimeout(
				() => reject(new Error(`${label} timed out after ${MCP_REFRESH_TIMEOUT_MS / 1000}s`)),
				MCP_REFRESH_TIMEOUT_MS
			);
		});
		try {
			return await Promise.race([promise, timeout]);
		} finally {
			if (timeoutId) clearTimeout(timeoutId);
		}
	}

	/**
	 * Re-reads persisted MCP settings (`settingsStore.settings.cometmind.mcp`)
	 * into the local editing draft, but only for servers the user is not
	 * actively editing (expanded). This is the fix for the "toggle MCP off/on,
	 * click Refresh status, it's still stuck" report: previously
	 * refreshMcpRuntime() only pulled runtime *connection* status
	 * (listMcpServers/listMcpTools) and never looked at whether the on-disk
	 * `enabled` flags actually matched the draft, so Refresh could never
	 * self-heal a draft that silently reverted (unsaved-changes discard, panel
	 * remount, etc.) — it always looked "successful" while showing stale data.
	 *
	 * The expanded server (if any) is left alone so this cannot clobber
	 * in-progress edits the user hasn't saved yet.
	 */
	function resyncDraftFromPersistedSettings() {
		const persisted = settingsStore.settings.cometmind.mcp;
		const persistedById = new Map(persisted.servers.map((server) => [server.id, server]));
		const nextServers = mcp.servers.map((server) => {
			if (server.id === expandedServerId) return server;
			const saved = persistedById.get(server.id);
			return saved ? { ...server, ...saved } : server;
		});
		for (const saved of persisted.servers) {
			if (!nextServers.some((server) => server.id === saved.id)) {
				nextServers.push(saved);
			}
		}
		updateMcp({ enabled: persisted.enabled, servers: nextServers });
	}

	async function refreshMcpRuntime({ silent = false }: { silent?: boolean } = {}) {
		if (silent) {
			autoRefreshInFlight = true;
		} else {
			mcpBusy = true;
			mcpStatus = '';
		}
		try {
			resyncDraftFromPersistedSettings();
			const [servers, tools] = await Promise.all([
				withTimeout(listMcpServers(), 'MCP server status'),
				withTimeout(listMcpTools(), 'MCP tool list')
			]);
			serverStatuses = servers ?? [];
			toolPreview = tools ?? [];
			rememberDiscoveredTools();
			if (servers?.some((server) => server.status === 'reloading')) {
				mcpStatus = MCP_RELOADING_STATUS_MESSAGE;
			} else if (mcpStatus === MCP_RELOADING_STATUS_MESSAGE) {
				mcpStatus = '';
			}
		} catch (err) {
			mcpStatus = err instanceof Error ? err.message : 'Failed to load MCP status';
		} finally {
			if (silent) {
				autoRefreshInFlight = false;
			} else {
				mcpBusy = false;
			}
		}
	}

	async function onReconnectServer(serverId: string) {
		setRowBusy(serverId, 'reconnect');
		mcpStatus = '';
		try {
			await reconnectMcpServer(serverId);
			mcpStatus = `Reconnected. Save settings if you changed configuration.`;
			await refreshMcpRuntime({ silent: true });
		} catch (err) {
			mcpStatus = apiErrorMessage(err, 'Reconnect failed');
		} finally {
			setRowBusy(serverId, null);
		}
	}

	async function onTestServer(serverId: string) {
		setRowBusy(serverId, 'test');
		mcpStatus = '';
		try {
			const result = await testMcpServer(serverId);
			if (result.ok) {
				const byName = new Map(
					(knownToolsByServer[serverId] ?? []).map((tool) => [tool.name, tool.description])
				);
				for (const name of result.tools ?? []) {
					const clean = name.trim();
					if (clean && !byName.has(clean)) byName.set(clean, '');
				}
				knownToolsByServer = {
					...knownToolsByServer,
					[serverId]: [...byName.entries()]
						.map(([name, description]) => ({ name, description }))
						.sort((a, b) => a.name.localeCompare(b.name))
				};
				mcpStatus = `Test ok · ${result.tool_count} tool${result.tool_count === 1 ? '' : 's'}.`;
			} else {
				mcpStatus = result.error_hint || result.error || 'Test failed';
			}
		} catch (err) {
			mcpStatus = apiErrorMessage(err, 'Test failed');
		} finally {
			setRowBusy(serverId, null);
		}
	}

	async function onConnectOAuth(server: MCPServerConfig) {
		if (!String(server.url ?? '').trim()) {
			mcpStatus = 'Set the server URL before connecting with OAuth.';
			return;
		}
		setRowBusy(server.id, 'oauth');
		mcpStatus = 'Saving settings before OAuth connect…';
		try {
			syncServerLists(server.id);
			normalizeConnection(server.id);
			await onPersistBeforeRuntimeAction?.({ mcp });
			mcpStatus = 'Opening your browser to authorize. Complete sign-in, then return here…';
			// CometMind drives the entire OAuth flow: metadata discovery, dynamic
			// client registration, browser authorization (loopback capture), token
			// exchange, and reconnect. No manual client ID / URLs required.
			const result = await startMcpOAuth(server.id);
			if (result?.connected === false) {
				mcpStatus =
					result.error_hint ||
					result.error ||
					'Signed in, but the MCP handshake did not finish. Click Reconnect.';
			} else {
				mcpStatus = 'Connected with OAuth.';
			}
			await refreshMcpRuntime({ silent: true });
		} catch (err) {
			mcpStatus = apiErrorMessage(err, 'OAuth connect failed');
		} finally {
			setRowBusy(server.id, null);
		}
	}

	export function syncFields() {
		mcp = {
			...mcp,
			servers: (mcp.servers ?? []).map((server) => ({
				...server,
				args: (argsTexts[server.id] ?? '')
					.split(/\s+/)
					.map((part) => part.trim())
					.filter(Boolean),
				env: parseEnv(envTexts[server.id] ?? ''),
				headers: parseEnv(headerTexts[server.id] ?? ''),
				oauth: server.oauth
			}))
		};
	}

	// Parses the local text mirrors (argsTexts / envTexts / headerTexts) into the
	// bound `mcp` draft for one server. Called from `oninput` (not just
	// change/blur) so editing these fields enables the Save button immediately;
	// otherwise the first Save click can land while the button is still disabled.
	// Safe to call per keystroke: the inputs bind to the text maps, not to this
	// parsed output, and it does not change the server-id set so the
	// syncTextFieldsFromSettings() $effect won't clobber in-flight edits.
	// See docs/postmortem/settings-save-disabled-state-mutation-in-derived.md.
	function syncServerLists(serverId: string) {
		updateServer(serverId, {
			args: (argsTexts[serverId] ?? '')
				.split(/\s+/)
				.map((part) => part.trim())
				.filter(Boolean),
			env: parseEnv(envTexts[serverId] ?? ''),
			headers: parseEnv(headerTexts[serverId] ?? '')
		});
	}

	/**
	 * For HTTP/SSE servers, fold a misplaced API-key header into the URL query
	 * string. Some servers (e.g. Typefully) only accept the key as a query
	 * parameter or `Authorization: Bearer`, and silently reject a custom header
	 * named after the key with HTTP 400 ("Bad Request" on initialize).
	 */
	function normalizeConnection(serverId: string) {
		const current = mcp.servers.find((server) => server.id === serverId);
		if (!current) return;
		const normalized = normalizeServerConnection(current);
		if (normalized === current) return;
		updateServer(serverId, { url: normalized.url, headers: normalized.headers });
		headerTexts = setTextField(headerTexts, serverId, formatEnv(normalized.headers));
		const moved = Object.keys(current.headers ?? {}).filter(
			(name) => !(name in (normalized.headers ?? {}))
		);
		if (moved.length > 0) {
			mcpStatus = `Moved ${moved.join(', ')} into the server URL (this server authenticates via the URL).`;
		}
	}
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>MCP servers</h3>
		<p>
			Connect external tool servers so CometMind can search, browse, and interact with
			services beyond built-in tools. You can also add servers manually under
			<code>cometmind.mcp</code> in <code>~/.cometmind/cometline-settings.json</code>.
		</p>
	</div>

	<SettingsToggle
		label="Use MCP tools in chat"
		description="Discover tools from configured servers when the sidecar starts."
		checked={mcp.enabled}
		onchange={(enabled) => updateMcp({ enabled })}
	/>

	{#if mcpStatus}
		<p
			class="settings-field-hint"
			class:error={mcpStatus.toLowerCase().includes('fail') ||
				mcpStatus.toLowerCase().includes('invalid')}
		>
			{mcpStatus}
		</p>
	{/if}

	<div class="mcp-toolbar">
		<SettingsButton variant="secondary" onclick={addServer}>
			<Plus size={14} strokeWidth={2} />
			Add server
		</SettingsButton>
		<SettingsButton variant="secondary" disabled={mcpBusy} onclick={importFromCursor}>
			<Download size={14} strokeWidth={2} />
			Import from Cursor
		</SettingsButton>
		<SettingsButton variant="secondary" disabled={mcpBusy} onclick={() => refreshMcpRuntime()}>
			<RefreshCw size={14} strokeWidth={2} class={mcpBusy ? 'spin' : ''} />
			{mcpBusy ? 'Refreshing…' : 'Refresh status'}
		</SettingsButton>
	</div>

	<div class="mcp-server-list">
		<div class="mcp-server-list-header">
			<span>Configured servers</span>
			<strong>{mcp.servers.length}</strong>
		</div>

		{#if mcp.servers.length === 0}
			<p class="settings-field-hint mcp-list-empty">No servers configured yet.</p>
		{:else}
			{#each mcp.servers as server (server.id)}
				{@const status = statusFor(server.id)}
				{@const expanded = expandedServerId === server.id}
				<div class="mcp-server-item" class:expanded>
					<div class="mcp-server-row-wrap">
						<button
							type="button"
							class="mcp-server-row"
							aria-expanded={expanded}
							onclick={() => toggleExpanded(server.id)}
						>
							<span class="row-chevron" aria-hidden="true">
								{#if expanded}
									<ChevronDown size={16} strokeWidth={2} />
								{:else}
									<ChevronRight size={16} strokeWidth={2} />
								{/if}
							</span>
							<span class="row-main">
								<span class="row-title">
									<strong>{server.name}</strong>
									<span class="status-badge {statusClass(status, server)}">
										{statusLabel(status, server)}
										{#if status?.tool_count}
											· {status.tool_count} tools
										{/if}
									</span>
								</span>
								<span class="row-summary">{connectionSummary(server)}</span>
							</span>
						</button>
						<div class="mcp-server-actions">
							<label class="row-toggle" title="Enable this server">
								<input
									type="checkbox"
									checked={server.enabled}
									onchange={(e) =>
										updateServer(server.id, {
											enabled: e.currentTarget.checked
										})}
								/>
								<span>On</span>
							</label>
							<button
								type="button"
								class="row-remove"
								aria-label={`Remove ${server.name}`}
								title="Remove server"
								onclick={(e) => {
									e.stopPropagation();
									removeServer(server.id);
								}}
							>
								<Trash2 size={14} />
							</button>
						</div>
					</div>

					{#if displayError(status) && !expanded}
						<p class="row-error">{displayError(status)}</p>
					{/if}

					{#if expanded}
						<div class="mcp-server-editor">
							<SettingsField label="Display name">
								<input
									type="text"
									value={server.name}
									oninput={(e) =>
										updateServer(server.id, { name: e.currentTarget.value })}
								/>
							</SettingsField>

							<SettingsField
								label="Connection type"
								note={transportHint(server.transport)}
							>
								<select
									value={server.transport}
									onchange={(e) =>
										updateServer(server.id, {
											transport: e.currentTarget.value as MCPTransport
										})}
								>
									{#each transportOptions as option (option.value)}
										<option value={option.value}>{option.label}</option>
									{/each}
								</select>
							</SettingsField>

							{#if server.transport === 'stdio'}
								<SettingsField label="Command">
									<input
										type="text"
										value={server.command ?? ''}
										oninput={(e) =>
											updateServer(server.id, {
												command: e.currentTarget.value
											})}
										placeholder="npx"
										spellcheck="false"
									/>
								</SettingsField>
								<SettingsField
									label="Arguments"
									note="Space-separated command arguments."
								>
									<input
										type="text"
										value={argsTexts[server.id] ?? ''}
										oninput={(e) => {
											argsTexts = setTextField(
												argsTexts,
												server.id,
												e.currentTarget.value
											);
											syncServerLists(server.id);
										}}
										onchange={() => syncServerLists(server.id)}
										onblur={() => syncServerLists(server.id)}
										placeholder="-y @modelcontextprotocol/server-filesystem /path/to/dir"
										spellcheck="false"
									/>
								</SettingsField>
							{:else}
								<SettingsField
									label="Server URL"
									note="Paste the full MCP URL. Known hosts (e.g. Atlassian) are rewritten to the current endpoint on blur. API keys that belong in the query string stay here, not in a header."
								>
									<input
										type="text"
										value={server.url ?? ''}
										oninput={(e) =>
											updateServer(server.id, { url: e.currentTarget.value })}
										onblur={() => normalizeConnection(server.id)}
										placeholder="https://example.com/mcp"
										spellcheck="false"
									/>
								</SettingsField>
							{/if}

							{#if server.enabled && mcp.enabled}
								<div class="editor-actions">
									<SettingsButton
										variant="secondary"
										disabled={isRowBusy(server.id)}
										onclick={() => onTestServer(server.id)}
									>
										{rowBusy[server.id] === 'test' ? 'Testing…' : 'Test'}
									</SettingsButton>
									<SettingsButton
										variant="secondary"
										disabled={isRowBusy(server.id)}
										onclick={() => onReconnectServer(server.id)}
									>
										{rowBusy[server.id] === 'reconnect'
											? 'Reconnecting…'
											: 'Reconnect'}
									</SettingsButton>
								</div>
							{/if}

							{#if displayError(status, true)}
								<p class="settings-field-hint error">{displayError(status, true)}</p>
							{/if}

							{#if server.transport === 'stdio'}
								<SettingsField
									label="Environment variables"
									note="One KEY=value per line."
								>
									<textarea
										value={envTexts[server.id] ?? ''}
										oninput={(e) => {
											envTexts = setTextField(
												envTexts,
												server.id,
												e.currentTarget.value
											);
											syncServerLists(server.id);
										}}
										onchange={() => syncServerLists(server.id)}
										onblur={() => syncServerLists(server.id)}
										rows="3"
										spellcheck="false"
									></textarea>
								</SettingsField>
							{:else}
								<SettingsField
									label="Headers"
									note="One KEY=value per line. Use Authorization=Bearer … for token auth. A key whose name matches the URL's query parameter is moved into the URL automatically."
								>
									<textarea
										value={headerTexts[server.id] ?? ''}
										oninput={(e) => {
											headerTexts = setTextField(
												headerTexts,
												server.id,
												e.currentTarget.value
											);
											syncServerLists(server.id);
										}}
										onchange={() => syncServerLists(server.id)}
										onblur={() => {
											syncServerLists(server.id);
											normalizeConnection(server.id);
										}}
										rows="3"
										spellcheck="false"
									></textarea>
								</SettingsField>

								<div class="oauth-block">
									<p class="advanced-label">OAuth</p>
									<p class="settings-field-hint">
										For servers that require sign-in (e.g. Atlassian), click
										Connect to authorize in your browser. Cometline handles
										discovery and registration automatically — no client ID or
										URLs needed. Tokens are stored in
										<code>~/.cometmind/mcp-oauth/</code>, not in settings JSON.
									</p>
									<div class="oauth-actions">
										<SettingsButton
											variant="secondary"
											disabled={isRowBusy(server.id)}
											onclick={() => onConnectOAuth(server)}
										>
											{rowBusy[server.id] === 'oauth'
												? 'Connecting…'
												: 'Connect with OAuth'}
										</SettingsButton>
									<span class="oauth-status">
										{status?.oauth_connected
											? status?.status === 'connected'
												? 'Signed in · connected'
												: 'Signed in'
											: 'Not signed in'}
									</span>
									</div>
								</div>
							{/if}

							<SettingsField
								label="Allowed tools"
								note="Turn tools off to hide them from the agent. All on (the default) exposes every tool."
							>
								<div class="tool-toggles">
									{#each knownToolsFor(server) as tool (tool.name)}
											<button
												type="button"
												class="tool-toggle"
												role="switch"
												aria-checked={isToolAllowed(server, tool.name)}
												onclick={() => toggleTool(server.id, tool.name)}
											>
												<input
													type="checkbox"
													tabindex="-1"
													checked={isToolAllowed(server, tool.name)}
													onclick={(e) => e.preventDefault()}
												/>
												<span class="tool-toggle-text">
													<strong>{tool.name}</strong>
													{#if tool.description}
														<span class="tool-toggle-desc"
															>{tool.description}</span
														>
													{/if}
												</span>
											</button>
									{:else}
									<p class="settings-field-hint">
										No tools discovered yet. Save settings, then use Test
										connection to load this server's tools — they'll appear here
										as toggles.
									</p>
									{/each}
								</div>
							</SettingsField>
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</div>

	{#if (toolPreview ?? []).length > 0}
		<p class="settings-field-hint mcp-footnote">
			{(toolPreview ?? []).length} tool(s) registered across all servers. Save settings to apply
			changes.
		</p>
	{/if}
</div>

<style>
	.mcp-list-empty {
		margin: 0;
		padding: 12px 11px;
	}

	.mcp-toolbar,
	.editor-actions,
	.oauth-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}

	.mcp-server-list {
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
		overflow: hidden;
	}

	.mcp-server-list-header {
		display: flex;
		justify-content: space-between;
		padding: 9px 11px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 248, 244, 0.94);
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
	}

	.mcp-server-item {
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
	}

	.mcp-server-item:last-child {
		border-bottom: 0;
	}

	.mcp-server-row-wrap {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: center;
		gap: 8px;
		padding: 10px 11px;
	}

	.mcp-server-actions {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
	}

	.row-remove {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 1px solid transparent;
		background: transparent;
		color: var(--text-muted);
		border-radius: 8px;
		padding: 6px;
		cursor: pointer;
	}

	.row-remove:hover {
		color: var(--status-error);
		background: rgba(180, 35, 24, 0.08);
		border-color: rgba(180, 35, 24, 0.18);
	}

	.mcp-server-row-wrap:hover {
		background: rgba(15, 23, 42, 0.06);
	}

	.mcp-server-item.expanded .mcp-server-row-wrap {
		background: rgba(15, 23, 42, 0.04);
	}

	.mcp-server-row {
		min-width: 0;
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 8px 10px;
		align-items: center;
		padding: 0;
		border: none;
		background: transparent;
		text-align: left;
		cursor: pointer;
		font: inherit;
		color: inherit;
	}

	.mcp-server-row:hover {
		background: transparent;
	}

	.row-chevron {
		display: flex;
		color: var(--text-muted);
	}

	.row-main {
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 3px;
	}

	.row-title {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
	}

	.row-title strong {
		font-size: 13px;
	}

	.row-summary {
		font-size: 11px;
		color: var(--text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.row-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding-right: 2px;
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
		cursor: pointer;
		flex-shrink: 0;
	}

	.row-error {
		margin: 0;
		padding: 0 11px 8px 37px;
		font-size: 11px;
		color: var(--status-error);
	}

	.mcp-server-editor {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 0 11px 14px 37px;
	}

	.status-badge {
		display: inline-block;
		padding: 1px 8px;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 650;
		text-transform: capitalize;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.status-badge.connected {
		background: rgba(47, 111, 79, 0.12);
		color: #2f6f4f;
	}

	.status-badge.error {
		background: rgba(180, 35, 24, 0.12);
		color: var(--status-error);
	}

	.status-badge.pending {
		background: rgba(180, 130, 24, 0.14);
		color: #8a5a10;
	}

	.tool-toggles {
		display: flex;
		flex-direction: column;
		gap: 2px;
		border: 1px solid rgba(0, 0, 0, 0.08);
		border-radius: 10px;
		overflow: hidden;
	}

	.tool-toggle {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		width: 100%;
		margin: 0;
		padding: 8px 10px;
		border: 0;
		border-bottom: 1px solid rgba(0, 0, 0, 0.05);
		background: transparent;
		text-align: left;
		font: inherit;
		color: inherit;
		cursor: pointer;
	}

	.tool-toggle:last-child {
		border-bottom: 0;
	}

	.tool-toggle:hover {
		background: rgba(0, 0, 0, 0.03);
	}

	.tool-toggle input {
		flex: 0 0 auto;
		width: 14px;
		height: 14px;
		margin: 2px 0 0;
		pointer-events: none;
	}

	.tool-toggle-text {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.tool-toggle-text strong {
		font-size: 12px;
	}

	.tool-toggle-desc {
		font-size: 11px;
		color: var(--text-muted);
	}

	.advanced-label {
		margin: 0;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
	}

	.oauth-block {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.oauth-status {
		font-size: 11px;
		color: var(--text-muted);
	}

	.settings-field-hint.error,
	.settings-field-hint.error {
		color: var(--status-error);
	}

	.mcp-footnote {
		margin-top: 4px;
	}

	textarea,
	input,
	select {
		width: 100%;
	}

	textarea {
		resize: vertical;
		min-height: 72px;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 12px;
	}

	:global(.spin) {
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
