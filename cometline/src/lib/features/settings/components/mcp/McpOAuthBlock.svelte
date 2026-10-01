<script lang="ts">
	import SettingsButton from '../SettingsButton.svelte';
	import type { MCPServerConfig } from '$lib/cometmind-settings';
	import type { McpServerStatus } from '$lib/client/cometmind';
	import type { McpPanelController } from '$lib/features/settings/mcp-panel-controller.svelte';

	let {
		server,
		status,
		controller
	}: {
		server: MCPServerConfig;
		status: McpServerStatus | undefined;
		controller: McpPanelController;
	} = $props();
</script>

<div class="oauth-block">
	<p class="advanced-label">OAuth</p>
	<p class="settings-field-hint">
		For servers that require sign-in (e.g. Atlassian), click Connect to authorize in your
		browser. Cometline handles discovery and registration automatically — no client ID or URLs
		needed. Tokens are stored in
		<code>~/.cometmind/mcp-oauth/</code>, not in settings JSON.
	</p>
	<div class="oauth-actions">
		<SettingsButton
			variant="secondary"
			disabled={controller.isRowBusy(server.id)}
			onclick={() => controller.connectOAuth(server)}
		>
			{controller.rowBusy[server.id] === 'oauth' ? 'Connecting…' : 'Connect with OAuth'}
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

<style>
	.oauth-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
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
</style>
