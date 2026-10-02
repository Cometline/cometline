<script lang="ts">
	import SettingsPersistenceHint from './SettingsPersistenceHint.svelte';
	import {
		SECTION_PERSISTENCE_HINTS,
		type SettingsSection
	} from '#lib/features/settings/pending-settings.js';

	let { section }: { section: SettingsSection } = $props();

	const hints = $derived(SECTION_PERSISTENCE_HINTS[section]);
</script>

<div class="settings-tab-persistence" aria-label="How changes on this tab are saved">
	{#if hints.pending}
		<SettingsPersistenceHint tier="pending" detail={hints.pending} />
	{/if}
	{#if hints.instant}
		<SettingsPersistenceHint tier="instant" detail={hints.instant} />
	{/if}
	{#if hints.action}
		<SettingsPersistenceHint tier="action" detail={hints.action} />
	{/if}
</div>
