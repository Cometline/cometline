<script lang="ts">
	import SettingsMCPPanel from './SettingsMCPPanel.svelte';
	import {
		defaultCometMindMCPSettings,
		type CometMindMCPSettings
	} from '#lib/cometmind-settings.js';

	let {
		mcp: initialMcp,
		onPersistBeforeRuntimeAction
	}: {
		mcp?: CometMindMCPSettings;
		onPersistBeforeRuntimeAction?: (overrides?: { mcp: CometMindMCPSettings }) => Promise<void>;
	} = $props();

	// Intentional snapshot: the panel owns the draft after mount.
	// svelte-ignore state_referenced_locally
	let mcp = $state<CometMindMCPSettings>(initialMcp ?? defaultCometMindMCPSettings());
</script>

<SettingsMCPPanel bind:mcp {onPersistBeforeRuntimeAction} />
<p data-testid="mcp-enabled">{String(mcp.enabled)}</p>
<p data-testid="server-count">{mcp.servers.length}</p>
<p data-testid="server-ids">{mcp.servers.map((server) => server.id).join(',')}</p>
