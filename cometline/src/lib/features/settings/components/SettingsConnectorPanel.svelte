<script lang="ts">
	import SettingsButton from './SettingsButton.svelte';
	import SettingsMCPPanel from './SettingsMCPPanel.svelte';
	import type { CometMindMCPSettings, MCPServerConfig } from '#lib/cometmind-settings.js';
	import {
		connectorServerConfig,
		defaultConnectorServerId,
		loadConnectorCatalog,
		type DefaultConnector
	} from '#lib/features/settings/default-connectors.js';

	let {
		mcp = $bindable(),
		onPersistBeforeRuntimeAction
	}: {
		mcp: CometMindMCPSettings;
		onPersistBeforeRuntimeAction?: (overrides?: { mcp: CometMindMCPSettings }) => Promise<void>;
	} = $props();

	let mcpPanel = $state<SettingsMCPPanel | undefined>();
	let connectors = $state<DefaultConnector[]>([]);
	let catalogError = $state('');

	$effect(() => {
		let active = true;
		loadConnectorCatalog()
			.then((loaded) => {
				if (active) connectors = loaded;
			})
			.catch(() => {
				if (active) catalogError = 'The connector catalog could not be loaded.';
			});
		return () => {
			active = false;
		};
	});

	const configuredIds = $derived(new Set((mcp.servers ?? []).map((server) => server.id)));

	function isConnected(connector: DefaultConnector): boolean {
		return configuredIds.has(defaultConnectorServerId(connector.id));
	}

	function connect(connector: DefaultConnector) {
		if (isConnected(connector)) return;
		const server = connectorServerConfig(connector);
		mcp = {
			...mcp,
			enabled: true,
			servers: [...(mcp.servers ?? []), server]
		};
	}

	function disconnect(connector: DefaultConnector) {
		const id = defaultConnectorServerId(connector.id);
		mcp = {
			...mcp,
			servers: (mcp.servers ?? []).filter((server) => server.id !== id)
		};
	}

	function modeLabel(server: MCPServerConfig | undefined): string {
		if (!server) return '';
		return server.transport === 'http' ? 'Remote' : 'Local';
	}

	export function syncFields() {
		mcpPanel?.syncFields();
	}
</script>

<section class="connector-panel settings-panel-frame">
	<div class="settings-panel-body">
		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Default connectors</h3>
				<p>
					Enable verified MCP services without writing the configuration yourself. Connecting
					one creates a normal MCP server entry and does not remove your custom servers.
				</p>
			</div>

			{#if catalogError}
				<p class="catalog-error">{catalogError}</p>
			{/if}
			<div class="connector-list">
				{#each connectors as connector (connector.id)}
					{@const server = (mcp.servers ?? []).find(
						(item) => item.id === defaultConnectorServerId(connector.id)
					)}
					<article class="connector-card">
						<span class="connector-icon">{@html connector.iconSvg}</span>
						<div class="connector-copy">
							<h4>{connector.name}</h4>
							<p>{connector.description}</p>
							<p class="connector-meta">
								{connector.modes[0]?.id === 'local' ? 'Local' : 'Remote'}
								{#if server}
									· {modeLabel(server)} server configured
								{/if}
							</p>
						</div>
						{#if connector.modes.length === 0}
							<span class="connector-unavailable">Unavailable</span>
						{:else if isConnected(connector)}
							<SettingsButton variant="secondary" onclick={() => disconnect(connector)}>
								Disconnect
							</SettingsButton>
						{:else}
							<SettingsButton variant="secondary" onclick={() => connect(connector)}>
								Connect
							</SettingsButton>
						{/if}
					</article>
				{/each}
			</div>
		</div>

		<SettingsMCPPanel
			bind:this={mcpPanel}
			bind:mcp
			{onPersistBeforeRuntimeAction}
		/>
	</div>
</section>

<style>
	.connector-list {
		display: grid;
		gap: 8px;
	}

	.connector-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 12px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
	}

	.connector-icon {
		display: inline-flex;
		width: 28px;
		height: 28px;
		flex: 0 0 auto;
	}

	.connector-icon :global(svg) {
		width: 28px;
		height: 28px;
		fill: currentColor;
	}

	.catalog-error {
		margin: 0;
		color: var(--danger, #b42318);
		font-size: 13px;
	}

	.connector-copy {
		flex: 1;
		min-width: 0;
	}

	.connector-unavailable {
		flex: 0 0 auto;
		color: color-mix(in srgb, var(--text-main) 58%, transparent);
		font-size: 12px;
		font-weight: 700;
	}

	.connector-card h4,
	.connector-card p {
		margin: 0;
	}

	.connector-card h4 {
		font-size: 14px;
	}

	.connector-card p {
		margin-top: 3px;
		color: color-mix(in srgb, var(--text-main) 72%, transparent);
		font-size: 12px;
		line-height: 1.45;
	}

	.connector-meta {
		font-size: 11px;
	}

	@media (max-width: 720px) {
		.connector-card {
			align-items: flex-start;
			flex-direction: column;
		}
	}
</style>
