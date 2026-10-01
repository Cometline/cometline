<script lang="ts">
	import SettingsPanel from '$lib/features/settings/components/SettingsPanel.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
</script>

{#if shellStore.settingsOpen}
	<SettingsPanel />
{/if}
