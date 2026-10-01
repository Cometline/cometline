import type { CometMindMCPSettings, MCPServerConfig } from '$lib/cometmind-settings';
import {
	mergeImportedMcpServers,
	parseCursorMcpJson
} from '$lib/features/settings/cursor-mcp-import';
import { normalizeServerConnection } from '$lib/features/settings/mcp-url';
import {
	MCP_RELOADING_STATUS_MESSAGE,
	formatEnv,
	mergeKnownTools,
	mergePersistedServers,
	newServerId,
	parseServerTextFields,
	sameToolNames,
	toggledAllowedTools,
	withTimeout,
	type KnownMcpTool
} from '$lib/features/settings/mcp-panel-format';
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

export type McpRowOp = 'reconnect' | 'oauth' | 'test';
type TextFieldMap = Record<string, string>;

export interface McpPanelControllerDeps {
	getMcp: () => CometMindMCPSettings;
	setMcp: (mcp: CometMindMCPSettings) => void;
	getOnPersistBeforeRuntimeAction: () =>
		| ((overrides?: { mcp: CometMindMCPSettings }) => Promise<void>)
		| undefined;
}

export function createMcpPanelController(deps: McpPanelControllerDeps) {
	let serverStatuses = $state<McpServerStatus[]>([]);
	let toolPreview = $state<McpToolInfo[]>([]);
	let mcpBusy = $state(false);
	let rowBusy = $state<Record<string, McpRowOp>>({});
	let autoRefreshInFlight = $state(false);
	let mcpStatus = $state('');
	let envTexts = $state<TextFieldMap>({});
	let headerTexts = $state<TextFieldMap>({});
	let argsTexts = $state<TextFieldMap>({});
	let expandedServerId = $state<string | null>(null);
	/**
	 * Tools we have ever seen for a server during this Settings session, keyed by
	 * server id. Seeded from discovered tools (toolPreview) and each server's
	 * saved allow-list so toggles stay visible even after a tool is disallowed
	 * (the backend stops reporting disallowed tools). UI-only; not persisted.
	 */
	let knownToolsByServer = $state<Record<string, KnownMcpTool[]>>({});

	// Only re-seed local text mirrors when the server *id set* changes (add/remove/import).
	// syncServerLists() replaces the servers array on every env/args/header keystroke so
	// the effect re-runs often; guard so we do not clobber in-progress textarea input
	// (e.g. "FOO" before "=" was typed).
	let lastServerIdKey = '';

	function setTextField(map: TextFieldMap, id: string, value: string) {
		return { ...map, [id]: value };
	}

	function syncTextFieldsFromSettings() {
		const nextEnv: TextFieldMap = {};
		const nextHeaders: TextFieldMap = {};
		const nextArgs: TextFieldMap = {};
		for (const server of deps.getMcp().servers ?? []) {
			nextEnv[server.id] = formatEnv(server.env);
			nextHeaders[server.id] = formatEnv(server.headers);
			nextArgs[server.id] = (server.args ?? []).join(' ');
		}
		envTexts = nextEnv;
		headerTexts = nextHeaders;
		argsTexts = nextArgs;
	}

	function syncTextFieldsOnServerSetChange() {
		const key = (deps.getMcp().servers ?? []).map((server) => server.id).join('\0');
		if (key === lastServerIdKey) return;
		lastServerIdKey = key;
		syncTextFieldsFromSettings();
	}

	function updateMcp(patch: Partial<CometMindMCPSettings>) {
		deps.setMcp({ ...deps.getMcp(), ...patch });
	}

	function updateServer(serverId: string, patch: Partial<MCPServerConfig>) {
		updateMcp({
			servers: deps
				.getMcp()
				.servers.map((server) =>
					server.id === serverId ? { ...server, ...patch } : server
				)
		});
	}

	function addServer() {
		const servers = deps.getMcp().servers ?? [];
		const id = newServerId(servers);
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
			const existing = deps.getMcp().servers ?? [];
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
		updateMcp({ servers: deps.getMcp().servers.filter((server) => server.id !== serverId) });
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
	function knownToolsFor(server: MCPServerConfig): KnownMcpTool[] {
		return mergeKnownTools(knownToolsByServer[server.id] ?? [], [], server.allowedTools ?? []);
	}

	/**
	 * Fold the currently discovered tools (toolPreview) into the session-scoped
	 * known-tools memory. Called after the tool list refreshes. Disallowed tools
	 * drop out of the backend's discovered list, so once seen we keep them.
	 */
	function rememberDiscoveredTools() {
		let changed = false;
		const next = { ...knownToolsByServer };
		for (const server of deps.getMcp().servers ?? []) {
			const merged = mergeKnownTools(
				next[server.id] ?? [],
				toolsForServer(server.id),
				server.allowedTools ?? []
			);
			if (!sameToolNames(next[server.id] ?? [], merged)) {
				next[server.id] = merged;
				changed = true;
			}
		}
		if (changed) knownToolsByServer = next;
	}

	function toggleTool(serverId: string, toolName: string) {
		const server = deps.getMcp().servers.find((item) => item.id === serverId);
		if (!server) return;
		const known = (knownToolsByServer[serverId] ?? []).map((tool) => tool.name);
		const next = toggledAllowedTools(known, server.allowedTools ?? [], toolName);
		updateServer(serverId, { allowedTools: next });
	}

	function setRowBusy(serverId: string, op: McpRowOp | null) {
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
		updateMcp({
			enabled: persisted.enabled,
			servers: mergePersistedServers(
				deps.getMcp().servers,
				persisted.servers,
				expandedServerId
			)
		});
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

	async function reconnectServer(serverId: string) {
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

	async function testServer(serverId: string) {
		setRowBusy(serverId, 'test');
		mcpStatus = '';
		try {
			const result = await testMcpServer(serverId);
			if (result.ok) {
				knownToolsByServer = {
					...knownToolsByServer,
					[serverId]: mergeKnownTools(
						knownToolsByServer[serverId] ?? [],
						[],
						result.tools ?? []
					)
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

	async function connectOAuth(server: MCPServerConfig) {
		if (!String(server.url ?? '').trim()) {
			mcpStatus = 'Set the server URL before connecting with OAuth.';
			return;
		}
		setRowBusy(server.id, 'oauth');
		mcpStatus = 'Saving settings before OAuth connect…';
		try {
			syncServerLists(server.id);
			normalizeConnection(server.id);
			await deps.getOnPersistBeforeRuntimeAction()?.({ mcp: deps.getMcp() });
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

	function parsedTextFields(serverId: string) {
		return parseServerTextFields({
			args: argsTexts[serverId] ?? '',
			env: envTexts[serverId] ?? '',
			headers: headerTexts[serverId] ?? ''
		});
	}

	function syncFields() {
		const mcp = deps.getMcp();
		deps.setMcp({
			...mcp,
			servers: (mcp.servers ?? []).map((server) => ({
				...server,
				...parsedTextFields(server.id),
				oauth: server.oauth
			}))
		});
	}

	// Parses the local text mirrors (argsTexts / envTexts / headerTexts) into the
	// bound `mcp` draft for one server. Called from `oninput` (not just
	// change/blur) so editing these fields enables the Save button immediately;
	// otherwise the first Save click can land while the button is still disabled.
	// Safe to call per keystroke: the inputs bind to the text maps, not to this
	// parsed output, and it does not change the server-id set so the
	// syncTextFieldsOnServerSetChange() $effect won't clobber in-flight edits.
	function syncServerLists(serverId: string) {
		updateServer(serverId, parsedTextFields(serverId));
	}

	/**
	 * For HTTP/SSE servers, fold a misplaced API-key header into the URL query
	 * string. Some servers (e.g. Typefully) only accept the key as a query
	 * parameter or `Authorization: Bearer`, and silently reject a custom header
	 * named after the key with HTTP 400 ("Bad Request" on initialize).
	 */
	function normalizeConnection(serverId: string) {
		const current = deps.getMcp().servers.find((server) => server.id === serverId);
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

	function setArgsText(serverId: string, value: string) {
		argsTexts = setTextField(argsTexts, serverId, value);
	}

	function setEnvText(serverId: string, value: string) {
		envTexts = setTextField(envTexts, serverId, value);
	}

	function setHeaderText(serverId: string, value: string) {
		headerTexts = setTextField(headerTexts, serverId, value);
	}

	return {
		get serverStatuses() {
			return serverStatuses;
		},
		get toolPreview() {
			return toolPreview;
		},
		get mcpBusy() {
			return mcpBusy;
		},
		get rowBusy() {
			return rowBusy;
		},
		get autoRefreshInFlight() {
			return autoRefreshInFlight;
		},
		get mcpStatus() {
			return mcpStatus;
		},
		get envTexts() {
			return envTexts;
		},
		get headerTexts() {
			return headerTexts;
		},
		get argsTexts() {
			return argsTexts;
		},
		get expandedServerId() {
			return expandedServerId;
		},
		syncTextFieldsOnServerSetChange,
		updateMcp,
		updateServer,
		addServer,
		importFromCursor,
		removeServer,
		toggleExpanded,
		statusFor,
		knownToolsFor,
		toggleTool,
		isRowBusy,
		refreshMcpRuntime,
		reconnectServer,
		testServer,
		connectOAuth,
		syncFields,
		syncServerLists,
		normalizeConnection,
		setArgsText,
		setEnvText,
		setHeaderText
	};
}

export type McpPanelController = ReturnType<typeof createMcpPanelController>;
