<script lang="ts">
	import SettingsButton from '../SettingsButton.svelte';
	import SettingsField from '../SettingsField.svelte';
	import McpOAuthBlock from './McpOAuthBlock.svelte';
	import McpToolToggles from './McpToolToggles.svelte';
	import type { MCPServerConfig, MCPTransport } from '#lib/cometmind-settings.js';
	import type { McpServerStatus } from '#lib/client/cometmind.js';
	import type { McpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import { isCatalogLocked } from '#lib/features/settings/mcp-catalog.js';
	import {
		displayError,
		transportHint,
		transportOptions
	} from '#lib/features/settings/mcp-panel-format.js';

	let {
		server,
		status,
		mcpEnabled,
		controller
	}: {
		server: MCPServerConfig;
		status: McpServerStatus | undefined;
		mcpEnabled: boolean;
		controller: McpPanelController;
	} = $props();

	const locked = $derived(isCatalogLocked(server));
</script>

<div class="mcp-server-editor">
	{#if locked}
		<div class="catalog-lock-banner">
			<p class="settings-field-hint">
				Installed from the connector catalog (v{server.catalogVersion ?? '1'}). Fields stay
				in sync with catalog updates until you unlock.
			</p>
			<SettingsButton variant="secondary" onclick={() => controller.unlockServer(server.id)}>
				Unlock for editing
			</SettingsButton>
		</div>
	{/if}
	<SettingsField label="Display name">
		<input
			type="text"
			value={server.name}
			disabled={locked}
			oninput={(e) => controller.updateServer(server.id, { name: e.currentTarget.value })}
		/>
	</SettingsField>

	<SettingsField label="Connection type" note={transportHint(server.transport)}>
		<select
			value={server.transport}
			disabled={locked}
			onchange={(e) =>
				controller.updateServer(server.id, {
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
				disabled={locked}
				oninput={(e) =>
					controller.updateServer(server.id, {
						command: e.currentTarget.value
					})}
				placeholder="npx"
				spellcheck="false"
			/>
		</SettingsField>
		<SettingsField label="Arguments" note="Space-separated command arguments.">
			<input
				type="text"
				value={controller.argsTexts[server.id] ?? ''}
				disabled={locked}
				oninput={(e) => {
					controller.setArgsText(server.id, e.currentTarget.value);
					controller.syncServerLists(server.id);
				}}
				onchange={() => controller.syncServerLists(server.id)}
				onblur={() => controller.syncServerLists(server.id)}
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
				disabled={locked}
				oninput={(e) => controller.updateServer(server.id, { url: e.currentTarget.value })}
				onblur={() => controller.normalizeConnection(server.id)}
				placeholder="https://example.com/mcp"
				spellcheck="false"
			/>
		</SettingsField>
	{/if}

	{#if server.enabled && mcpEnabled}
		<div class="editor-actions">
			<SettingsButton
				variant="secondary"
				disabled={controller.isRowBusy(server.id)}
				onclick={() => controller.testServer(server.id)}
			>
				{controller.rowBusy[server.id] === 'test' ? 'Testing…' : 'Test'}
			</SettingsButton>
			<SettingsButton
				variant="secondary"
				disabled={controller.isRowBusy(server.id)}
				onclick={() => controller.reconnectServer(server.id)}
			>
				{controller.rowBusy[server.id] === 'reconnect' ? 'Reconnecting…' : 'Reconnect'}
			</SettingsButton>
		</div>
	{/if}

	{#if displayError(status, true)}
		<p class="settings-field-hint error">
			{displayError(status, true)}
		</p>
	{/if}

	{#if server.transport === 'stdio'}
		<SettingsField label="Environment variables" note="One KEY=value per line.">
			<textarea
				value={controller.envTexts[server.id] ?? ''}
				disabled={locked}
				oninput={(e) => {
					controller.setEnvText(server.id, e.currentTarget.value);
					controller.syncServerLists(server.id);
				}}
				onchange={() => controller.syncServerLists(server.id)}
				onblur={() => controller.syncServerLists(server.id)}
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
				value={controller.headerTexts[server.id] ?? ''}
				disabled={locked}
				oninput={(e) => {
					controller.setHeaderText(server.id, e.currentTarget.value);
					controller.syncServerLists(server.id);
				}}
				onchange={() => controller.syncServerLists(server.id)}
				onblur={() => {
					controller.syncServerLists(server.id);
					controller.normalizeConnection(server.id);
				}}
				rows="3"
				spellcheck="false"
			></textarea>
		</SettingsField>

		<McpOAuthBlock {server} {status} {controller} />
	{/if}

	<McpToolToggles {server} {controller} />
</div>

<style>
	.editor-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}

	.catalog-lock-banner {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px;
		border-radius: 10px;
		background: rgba(15, 23, 42, 0.04);
		border: 1px dashed var(--border-soft);
	}

	.mcp-server-editor {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 0 11px 14px 37px;
	}

	.settings-field-hint.error {
		color: var(--status-error);
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
</style>
