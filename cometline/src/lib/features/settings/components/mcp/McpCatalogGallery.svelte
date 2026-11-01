<script lang="ts">
	import SettingsButton from '../SettingsButton.svelte';
	import type { McpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import { catalogAuthLabel, catalogScopesLabel } from '#lib/features/settings/mcp-catalog.js';
	import {
		displayError,
		statusClass,
		statusLabel
	} from '#lib/features/settings/mcp-panel-format.js';

	let {
		controller,
		mcpEnabled
	}: {
		controller: McpPanelController;
		mcpEnabled: boolean;
	} = $props();
</script>

<div class="mcp-catalog">
	<div class="mcp-catalog-header">
		<h4>Connectors</h4>
		<p class="settings-field-hint">
			One-click remote MCP connectors. CometMind runs OAuth in your browser on port 1456 — no
			URLs to paste.
		</p>
	</div>

	{#if controller.catalogLoadError}
		<p class="settings-field-hint error">{controller.catalogLoadError}</p>
	{/if}

	<div class="mcp-catalog-grid">
		{#each controller.catalogEntries as entry (entry.id)}
			{@const installed = controller.catalogServerFor(entry)}
			{@const status = controller.catalogStatusFor(entry)}
			<article class="mcp-catalog-card">
				<div class="card-head">
					<strong>{entry.name}</strong>
					<span class="auth-badge">{catalogAuthLabel(entry)}</span>
				</div>
				<p class="card-desc">{entry.description}</p>
				<dl class="card-meta">
					<div>
						<dt>Transport</dt>
						<dd>{entry.transport}</dd>
					</div>
					<div>
						<dt>Auth</dt>
						<dd>{catalogAuthLabel(entry)}</dd>
					</div>
					<div class="scopes">
						<dt>Scopes</dt>
						<dd>{catalogScopesLabel(entry)}</dd>
					</div>
				</dl>
				{#if installed}
					<p class="card-status">
						<span class="status-badge {statusClass(status, installed, mcpEnabled)}">
							{statusLabel(status, installed, mcpEnabled)}
							{#if status?.tool_count}
								· {status.tool_count} tools
							{/if}
						</span>
					</p>
					{#if displayError(status)}
						<p class="card-error">{displayError(status)}</p>
					{/if}
					{#if status?.status === 'connected' && (status.tool_count ?? 0) === 0}
						<p class="card-error">Connected, but no tools were reported yet.</p>
					{/if}
				{/if}
				<SettingsButton
					variant="secondary"
					disabled={controller.isCatalogConnectBusy(entry.id) || controller.mcpBusy}
					onclick={() => controller.connectCatalogEntry(entry)}
				>
					{controller.isCatalogConnectBusy(entry.id)
						? 'Connecting…'
						: installed?.enabled
							? 'Reconnect'
							: 'Connect'}
				</SettingsButton>
			</article>
		{/each}
	</div>
</div>

<style>
	.mcp-catalog {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-bottom: 12px;
	}

	.mcp-catalog-header h4 {
		margin: 0;
		font-size: 13px;
		font-weight: 650;
	}

	.mcp-catalog-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: 10px;
	}

	.mcp-catalog-card {
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		padding: 11px;
		background: rgba(255, 255, 255, 0.62);
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.card-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.card-desc {
		margin: 0;
		font-size: 11px;
		color: var(--text-muted);
		line-height: 1.45;
	}

	.card-meta {
		margin: 0;
		display: grid;
		gap: 4px;
		font-size: 10px;
	}

	.card-meta div {
		display: grid;
		grid-template-columns: 64px 1fr;
		gap: 6px;
	}

	.card-meta dt {
		margin: 0;
		font-weight: 650;
		color: var(--text-muted);
	}

	.card-meta dd {
		margin: 0;
		color: var(--text-main);
	}

	.scopes dd {
		word-break: break-word;
	}

	.auth-badge {
		font-size: 10px;
		font-weight: 650;
		padding: 1px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.07);
		color: var(--text-muted);
		flex-shrink: 0;
	}

	.card-status {
		margin: 0;
	}

	.card-error {
		margin: 0;
		font-size: 11px;
		color: var(--status-error);
	}

	.settings-field-hint.error {
		color: var(--status-error);
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
		color: var(--color-2f6f4f);
	}

	.status-badge.error {
		background: rgba(180, 35, 24, 0.12);
		color: var(--status-error);
	}

	.status-badge.pending {
		background: rgba(180, 130, 24, 0.14);
		color: var(--color-8a5a10);
	}
</style>
