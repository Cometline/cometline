<script lang="ts">
	import type { McpServerStatus } from '#lib/client/cometmind.js';
	import type { MCPServerConfig } from '#lib/cometmind-settings.js';
	import { statusClass, statusLabel } from '#lib/features/settings/mcp-panel-format.js';

	let {
		status,
		server,
		mcpEnabled,
		toolCount
	}: {
		status: McpServerStatus | undefined;
		server: MCPServerConfig;
		mcpEnabled: boolean;
		toolCount?: number;
	} = $props();

	const label = $derived(statusLabel(status, server, mcpEnabled));
	const klass = $derived(statusClass(status, server, mcpEnabled));
</script>

<span class="mcp-status-badge {klass}">
	{label}
	{#if toolCount}
		· {toolCount} tools
	{/if}
</span>

<style>
	.mcp-status-badge {
		display: inline-block;
		padding: 1px 8px;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 650;
		text-transform: capitalize;
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
		color: var(--text-muted);
	}

	.mcp-status-badge.connected {
		background: color-mix(in srgb, var(--status-success) 14%, transparent);
		color: var(--status-success);
	}

	.mcp-status-badge.error {
		background: color-mix(in srgb, var(--status-error) 12%, transparent);
		color: var(--status-error);
	}

	.mcp-status-badge.pending {
		background: color-mix(in srgb, var(--status-warning) 16%, transparent);
		color: var(--status-warning);
	}

	.mcp-status-badge.idle {
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
		color: var(--text-muted);
	}
</style>
