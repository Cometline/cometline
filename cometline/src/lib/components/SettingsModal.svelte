<script lang="ts">
	import SettingsPanel from './SettingsPanel.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
</script>

{#if shellStore.settingsOpen}
	<SettingsPanel />
{/if}
