<script lang="ts">
	import SettingsToggle from './SettingsToggle.svelte';
	import SettingsButton from './SettingsButton.svelte';
	import McpServerItem from './mcp/McpServerItem.svelte';
	import { type CometMindMCPSettings } from '#lib/cometmind-settings.js';
	import { createMcpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import {
		MCP_RELOADING_POLL_MS,
		hasPendingServer,
		isMcpStatusError
	} from '#lib/features/settings/mcp-panel-format.js';
	import { Download, Plus, RefreshCw } from '@lucide/svelte';
	import { onMount } from 'svelte';

	let {
		mcp = $bindable(),
		onPersistBeforeRuntimeAction
	}: {
		mcp: CometMindMCPSettings;
		onPersistBeforeRuntimeAction?: (overrides?: { mcp: CometMindMCPSettings }) => Promise<void>;
	} = $props();

	const controller = createMcpPanelController({
		getMcp: () => mcp,
		setMcp: (next) => {
			mcp = next;
		},
		getOnPersistBeforeRuntimeAction: () => onPersistBeforeRuntimeAction
	});

	$effect(() => {
		controller.syncTextFieldsOnServerSetChange();
	});

	onMount(() => {
		void controller.refreshMcpRuntime();
	});

	$effect(() => {
		if (
			!hasPendingServer(controller.serverStatuses) ||
			controller.mcpBusy ||
			controller.autoRefreshInFlight
		)
			return;

		const timeoutId = setTimeout(() => {
			void controller.refreshMcpRuntime({ silent: true });
		}, MCP_RELOADING_POLL_MS);

		return () => clearTimeout(timeoutId);
	});

	export function syncFields() {
		controller.syncFields();
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
		onchange={(enabled) => controller.updateMcp({ enabled })}
	/>

	{#if controller.mcpStatus}
		<p class="settings-field-hint" class:error={isMcpStatusError(controller.mcpStatus)}>
			{controller.mcpStatus}
		</p>
	{/if}

	<div class="mcp-toolbar">
		<SettingsButton variant="secondary" onclick={controller.addServer}>
			<Plus size={14} strokeWidth={2} />
			Add server
		</SettingsButton>
		<SettingsButton
			variant="secondary"
			disabled={controller.mcpBusy}
			onclick={controller.importFromCursor}
		>
			<Download size={14} strokeWidth={2} />
			Import from Cursor
		</SettingsButton>
		<SettingsButton
			variant="secondary"
			disabled={controller.mcpBusy}
			onclick={() => controller.refreshMcpRuntime()}
		>
			<RefreshCw size={14} strokeWidth={2} class={controller.mcpBusy ? 'spin' : ''} />
			{controller.mcpBusy ? 'Refreshing…' : 'Refresh status'}
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
				<McpServerItem
					{server}
					status={controller.statusFor(server.id)}
					expanded={controller.expandedServerId === server.id}
					mcpEnabled={mcp.enabled}
					{controller}
				/>
			{/each}
		{/if}
	</div>

	{#if (controller.toolPreview ?? []).length > 0}
		<p class="settings-field-hint mcp-footnote">
			{(controller.toolPreview ?? []).length} tool(s) registered across all servers. Save settings
			to apply changes.
		</p>
	{/if}
</div>

<style>
	.mcp-list-empty {
		margin: 0;
		padding: 12px 11px;
	}

	.mcp-toolbar {
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

	.settings-field-hint.error {
		color: var(--status-error);
	}

	.mcp-footnote {
		margin-top: 4px;
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
