<script lang="ts">
	import { LoaderCircle } from '@lucide/svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import SettingsButton from '../SettingsButton.svelte';

	let {
		status,
		hasPendingChanges,
		modelsSectionWarning,
		saveDisabled,
		onDiscard,
		onSave
	}: {
		status: string;
		hasPendingChanges: boolean;
		modelsSectionWarning: string;
		saveDisabled: boolean;
		onDiscard: () => void;
		onSave: () => void;
	} = $props();
</script>

{#if settingsStore.error}
	<p class="message error">{settingsStore.error}</p>
{:else if status}
	<p class="message success">{status}</p>
{/if}

<footer>
	<p class="settings-footer-copy">
		{#if settingsStore.isSaving}
			Saving changes…
		{:else}
			{#if hasPendingChanges}<strong>Unsaved changes ·</strong>{/if}
			Save applies all tabs. Close without saving discards pending edits.
		{/if}
		{#if modelsSectionWarning}<span class="settings-footer-warning">{modelsSectionWarning}</span
			>{/if}
	</p>
	<SettingsButton variant="secondary" onclick={onDiscard}>Discard</SettingsButton>
	<SettingsButton variant="primary" onclick={onSave} disabled={saveDisabled}>
		{#if settingsStore.isSaving}<span class="spin"><LoaderCircle size={14} /></span>{/if}
		Save changes
	</SettingsButton>
</footer>

<style>
	footer {
		display: flex;
		align-items: center;
	}

	footer p,
	.message {
		margin: 0;
	}

	.settings-footer-warning {
		display: block;
		margin-top: 4px;
		color: var(--status-error);
		font-size: 12px;
	}

	footer p {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.message {
		flex-shrink: 0;
		padding: 0 2px 12px;
		font-size: 12px;
	}

	.message.error {
		color: var(--status-error);
	}

	.message.success {
		color: #027a48;
	}

	footer {
		position: sticky;
		bottom: 0;
		z-index: 2;
		flex-shrink: 0;
		justify-content: flex-end;
		gap: 8px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.96);
	}

	footer p {
		margin-right: auto;
	}
</style>
