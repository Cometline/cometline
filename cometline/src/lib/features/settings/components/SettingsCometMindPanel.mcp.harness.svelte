<script lang="ts">
	import SettingsConnectorPanel from './SettingsConnectorPanel.svelte';
	import { defaultCometMindSettings, type CometMindSettings } from '#lib/cometmind-settings.js';

	let cometmind = $state<CometMindSettings>(defaultCometMindSettings());
</script>

<SettingsConnectorPanel bind:mcp={cometmind.mcp} />

<p data-testid="mcp-enabled">{String(cometmind.mcp.enabled)}</p>
<p data-testid="server-count">{cometmind.mcp.servers.length}</p>
