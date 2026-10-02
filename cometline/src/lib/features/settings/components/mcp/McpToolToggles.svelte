<script lang="ts">
	import SettingsField from '../SettingsField.svelte';
	import type { MCPServerConfig } from '#lib/cometmind-settings.js';
	import type { McpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import { isToolAllowed } from '#lib/features/settings/mcp-panel-format.js';

	let {
		server,
		controller
	}: {
		server: MCPServerConfig;
		controller: McpPanelController;
	} = $props();
</script>

<SettingsField
	label="Allowed tools"
	note="Turn tools off to hide them from the agent. All on (the default) exposes every tool."
>
	<div class="tool-toggles">
		{#each controller.knownToolsFor(server) as tool (tool.name)}
			<button
				type="button"
				class="tool-toggle"
				role="switch"
				aria-checked={isToolAllowed(server, tool.name)}
				onclick={() => controller.toggleTool(server.id, tool.name)}
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
						<span class="tool-toggle-desc">{tool.description}</span>
					{/if}
				</span>
			</button>
		{:else}
			<p class="settings-field-hint">
				No tools discovered yet. Save settings, then use Test connection to load this
				server's tools — they'll appear here as toggles.
			</p>
		{/each}
	</div>
</SettingsField>

<style>
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
</style>
