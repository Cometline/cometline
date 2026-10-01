<script lang="ts">
	import SettingsCometMindPanel from './SettingsCometMindPanel.svelte';
	import { defaultCometMindSettings, type CometMindSettings } from '$lib/cometmind-settings';

	let cometmind = $state<CometMindSettings>(defaultCometMindSettings());
</script>

<SettingsCometMindPanel bind:cometmind providers={[]} />

<p data-testid="mcp-enabled">{String(cometmind.mcp.enabled)}</p>
<p data-testid="server-count">{cometmind.mcp.servers.length}</p>
