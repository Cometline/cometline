<script lang="ts">
	import SettingsMCPPanel from './SettingsMCPPanel.svelte';
	import { defaultCometMindMCPSettings, type CometMindMCPSettings } from '$lib/cometmind-settings';

	let {
		onPersistBeforeRuntimeAction
	}: { onPersistBeforeRuntimeAction?: (overrides?: { mcp: CometMindMCPSettings }) => Promise<void> } =
		$props();

	let mcp = $state<CometMindMCPSettings>(defaultCometMindMCPSettings());
</script>

<SettingsMCPPanel bind:mcp {onPersistBeforeRuntimeAction} />
<p data-testid="mcp-enabled">{String(mcp.enabled)}</p>
<p data-testid="server-count">{mcp.servers.length}</p>
<p data-testid="server-ids">{mcp.servers.map((server) => server.id).join(',')}</p>
