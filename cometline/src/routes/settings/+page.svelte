<script lang="ts">
	import { onMount } from 'svelte';
	import SettingsPanel from '$lib/components/settings/SettingsPanel.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';

	let ready = $state(false);
	let loadError = $state('');

	onMount(() => {
		void settingsStore
			.load()
			.then(() => {
				ready = true;
			})
			.catch((err) => {
				loadError = err instanceof Error ? err.message : 'Failed to load settings';
				ready = true;
			});
	});

	function closeSettingsWindow() {
		window.close();
	}
</script>

<svelte:head>
	<title>Settings - Cometline</title>
</svelte:head>

{#if ready}
	{#if loadError}
		<div class="settings-load-error" role="alert">
			<h1>Settings</h1>
			<p>{loadError}</p>
		</div>
	{:else}
		<SettingsPanel mode="window" onClose={closeSettingsWindow} />
	{/if}
{:else}
	<div class="settings-loading" role="status">Loading settings...</div>
{/if}

<style>
	.settings-loading,
	.settings-load-error {
		display: grid;
		place-content: center;
		min-height: 100vh;
		padding: 24px;
		background: var(--app-bg);
		color: var(--text-main);
	}

	.settings-load-error {
		gap: 8px;
		text-align: center;
	}

	.settings-load-error h1,
	.settings-load-error p {
		margin: 0;
	}
</style>
