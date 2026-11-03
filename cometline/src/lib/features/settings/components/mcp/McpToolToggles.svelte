<script lang="ts">
	import { Search } from '@lucide/svelte';
	import type { MCPServerConfig } from '#lib/cometmind-settings.js';
	import type { McpPanelController } from '#lib/features/settings/mcp-panel-controller.svelte.js';
	import {
		isToolAllowed,
		toolExposeMode,
		toolMatchesQuery
	} from '#lib/features/settings/mcp-panel-format.js';

	let {
		server,
		controller
	}: {
		server: MCPServerConfig;
		controller: McpPanelController;
	} = $props();

	let query = $state('');

	const tools = $derived(controller.knownToolsFor(server));
	const visible = $derived(tools.filter((tool) => toolMatchesQuery(tool, query)));
	const allOpen = $derived(toolExposeMode(server.allowedTools) === 'all');
</script>

<div class="tool-panel">
	<div class="tool-heading">
		<span>Allowed tools</span>
		<p>Turn tools off to hide them from the agent. All on (the default) exposes every tool.</p>
	</div>
	{#if tools.length > 0}
		<div class="tool-toolbar">
			<div class="tool-search">
				<Search size={14} stroke-width={2} aria-hidden="true" class="search-icon" />
				<input
					class="tool-search-input"
					type="search"
					bind:value={query}
					placeholder="Filter tools…"
					spellcheck="false"
					autocomplete="off"
					aria-label={`Filter tools for ${server.name}`}
				/>
			</div>
			<button
				type="button"
				class="tool-all-toggle"
				role="switch"
				aria-checked={allOpen}
				aria-label={allOpen ? 'Turn all tools off' : 'Turn all tools on'}
				onclick={() => controller.setAllTools(server.id, !allOpen)}
			>
				<input type="checkbox" tabindex="-1" checked={allOpen} onclick={(e) => e.preventDefault()} />
				<span>{allOpen ? 'All on' : 'All off'}</span>
			</button>
		</div>
	{/if}

	<div class="tool-toggles">
		{#each visible as tool (tool.name)}
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
				{#if tools.length === 0}
					No tools discovered yet. Save settings, then use Test connection to load this
					server's tools — they'll appear here as toggles.
				{:else}
					No tools match “{query.trim()}”.
				{/if}
			</p>
		{/each}
	</div>
</div>

<style>
	.tool-heading {
		display: grid;
		gap: 4px;
	}

	.tool-heading span {
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
	}

	.tool-heading p {
		margin: 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-soft);
	}

	.tool-panel {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin: 2px 14px 14px;
		padding: 12px;
		border: 1px solid rgba(0, 0, 0, 0.06);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.45);
	}

	.tool-toolbar {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.tool-search {
		display: flex;
		flex: 1;
		flex-direction: row;
		align-items: center;
		gap: 8px;
		min-width: 0;
		height: 34px;
		margin: 0;
		padding: 0 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.76);
		color: var(--text-muted);
		font-size: 12px;
	}

	.tool-search:focus-within {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
		color: var(--text-main);
	}

	.tool-search :global(.search-icon) {
		flex: 0 0 14px;
		width: 14px;
		height: 14px;
	}

	.tool-search input.tool-search-input {
		flex: 1 1 auto;
		width: auto;
		min-width: 0;
		height: 32px;
		margin: 0;
		padding: 0;
		border: 0;
		border-radius: 0;
		background: transparent;
		box-shadow: none;
		font: inherit;
		font-size: 12px;
		line-height: 32px;
		color: inherit;
		outline: none;
	}

	.tool-search input.tool-search-input:focus {
		border: 0;
		outline: none;
		box-shadow: none;
	}

	.tool-search-input::-webkit-search-decoration,
	.tool-search-input::-webkit-search-cancel-button,
	.tool-search-input::-webkit-search-results-button {
		display: none;
	}

	.tool-search-input::placeholder {
		color: var(--text-muted);
	}

	.tool-all-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: 0 0 auto;
		height: 34px;
		margin: 0;
		padding: 0 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.76);
		font: inherit;
		font-size: 12px;
		color: inherit;
		cursor: pointer;
	}

	.tool-all-toggle:hover {
		background: rgba(0, 0, 0, 0.03);
	}

	.tool-all-toggle input {
		width: 14px;
		height: 14px;
		margin: 0;
		pointer-events: none;
	}

	.tool-toggles {
		display: flex;
		flex-direction: column;
		gap: 0;
		height: 280px;
		border: 1px solid rgba(0, 0, 0, 0.08);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.8);
		overflow-x: hidden;
		overflow-y: auto;
	}

	.tool-toggle {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		width: 100%;
		margin: 0;
		padding: 12px 14px;
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
		gap: 4px;
		min-width: 0;
	}

	.tool-toggle-text strong {
		font-size: 12px;
	}

	.tool-toggle-desc {
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.settings-field-hint {
		margin: 0;
		padding: 8px 10px;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-soft);
	}
</style>
