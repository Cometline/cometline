<script lang="ts">
	import { ChevronDown, ChevronRight, Trash2 } from '@lucide/svelte';
	import McpServerEditor from './McpServerEditor.svelte';
	import McpStatusBadge from './McpStatusBadge.svelte';
	import type { MCPServerConfig } from '#lib/cometmind-settings.js';
	import type { McpServerStatus } from '#lib/client/cometmind.js';
	import type { McpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import { connectionSummary, displayError } from '#lib/features/settings/mcp-panel-format.js';

	let {
		server,
		status,
		expanded,
		mcpEnabled,
		controller
	}: {
		server: MCPServerConfig;
		status: McpServerStatus | undefined;
		expanded: boolean;
		mcpEnabled: boolean;
		controller: McpPanelController;
	} = $props();
</script>

<div class="mcp-server-item" class:expanded>
	<div class="mcp-server-row-wrap">
		<button
			type="button"
			class="mcp-server-row"
			aria-expanded={expanded}
			onclick={() => controller.toggleExpanded(server.id)}
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
					<McpStatusBadge {status} {server} {mcpEnabled} toolCount={status?.tool_count} />
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
						controller.updateServer(server.id, {
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
					controller.removeServer(server.id);
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
		<McpServerEditor {server} {status} {mcpEnabled} {controller} />
	{/if}
</div>

<style>
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
</style>
