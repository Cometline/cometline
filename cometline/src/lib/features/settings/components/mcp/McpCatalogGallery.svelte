<script lang="ts">
	import McpConnectorIcon from './McpConnectorIcon.svelte';
	import type { McpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import {
		catalogMetaLine,
		isCatalogEntryConnected
	} from '#lib/features/settings/mcp-catalog.js';
	import { displayError } from '#lib/features/settings/mcp-panel-format.js';

	let {
		controller,
		mcpEnabled
	}: {
		controller: McpPanelController;
		mcpEnabled: boolean;
	} = $props();

	function rowActionLabel(
		entryId: string,
		installed: ReturnType<McpPanelController['catalogServerFor']>,
		connected: boolean
	): string {
		if (controller.isCatalogConnectBusy(entryId)) return 'Connecting…';
		if (connected) return 'Connected';
		if (installed?.enabled) return 'Connect';
		return 'Connect';
	}
</script>

<section class="connectors-panel" aria-labelledby="connectors-heading">
	<header class="connectors-hero">
		<p class="connectors-eyebrow">Connectors</p>
		<h4 id="connectors-heading" class="connectors-title">Manage remote MCP connectors</h4>
		<p class="settings-field-hint connectors-subtitle">
			One-click sign-in via your browser (port 1456). Only the catalog below — no extra
			plugins.
		</p>
	</header>

	{#if controller.catalogLoadError}
		<p class="settings-field-hint error">{controller.catalogLoadError}</p>
	{/if}

	{#if controller.catalogEntries.length > 0}
		<p class="connectors-section-label">Available</p>
		<ul class="connectors-grid">
			{#each controller.catalogEntries as entry (entry.id)}
				{@const installed = controller.catalogServerFor(entry)}
				{@const status = controller.catalogStatusFor(entry)}
				{@const connected = isCatalogEntryConnected(installed, status, mcpEnabled)}
				{@const busy = controller.isCatalogConnectBusy(entry.id)}
				{@const actionLabel = rowActionLabel(entry.id, installed, connected)}
				<li class="connector-row">
					<div class="connector-main">
						<McpConnectorIcon catalogId={entry.id} />
						<div class="connector-copy">
							<span class="connector-name">{entry.name}</span>
							<span class="connector-meta">
								{catalogMetaLine(entry, status, connected)}
							</span>
							{#if installed && displayError(status)}
								<span class="connector-error">{displayError(status)}</span>
							{/if}
							{#if connected && (status?.tool_count ?? 0) === 0}
								<span class="connector-error"
									>Connected, but no tools reported yet.</span
								>
							{/if}
						</div>
					</div>
					<div class="connector-action">
						{#if connected}
							<span class="status-connected">{actionLabel}</span>
						{:else if busy}
							<span class="status-pending">{actionLabel}</span>
						{:else}
							<button
								type="button"
								class="connect-link"
								disabled={controller.mcpBusy}
								onclick={() => controller.connectCatalogEntry(entry)}
							>
								{actionLabel}
							</button>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.connectors-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		margin: 4px 0 18px;
		padding: 0;
	}

	.connectors-hero {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.connectors-eyebrow {
		margin: 0;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
		letter-spacing: 0.01em;
	}

	.connectors-title {
		margin: 0;
		font-size: 22px;
		font-weight: 700;
		line-height: 1.2;
		color: var(--text-main);
		letter-spacing: -0.02em;
	}

	.connectors-subtitle {
		margin: 2px 0 0;
		max-width: 52ch;
	}

	.connectors-section-label {
		margin: 4px 0 0;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.connectors-grid {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		grid-template-columns: 1fr 1fr;
		column-gap: 28px;
		row-gap: 2px;
	}

	@media (max-width: 720px) {
		.connectors-grid {
			grid-template-columns: 1fr;
		}
	}

	.connector-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		min-height: 56px;
		padding: 8px 0;
		border: none;
		background: transparent;
	}

	.connector-main {
		display: flex;
		align-items: center;
		gap: 12px;
		min-width: 0;
		flex: 1;
	}

	.connector-copy {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.connector-name {
		font-size: 14px;
		font-weight: 650;
		color: var(--text-main);
		line-height: 1.25;
	}

	.connector-meta {
		font-size: 12px;
		color: var(--text-muted);
		line-height: 1.35;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.connector-error {
		font-size: 11px;
		color: var(--status-error);
		line-height: 1.35;
	}

	.connector-action {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: flex-end;
		min-width: 88px;
	}

	.status-connected {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-2f6f4f);
	}

	.status-pending {
		font-size: 13px;
		font-weight: 500;
		color: var(--text-muted);
	}

	.connect-link {
		border: none;
		background: transparent;
		padding: 4px 0;
		font: inherit;
		font-size: 13px;
		font-weight: 600;
		color: var(--text-main);
		cursor: pointer;
		text-decoration: none;
	}

	.connect-link:hover:not(:disabled) {
		text-decoration: underline;
	}

	.connect-link:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.settings-field-hint.error {
		color: var(--status-error);
	}
</style>
