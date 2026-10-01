<script lang="ts">
	import SettingsToggle from '../SettingsToggle.svelte';
	import { runStorageBackup } from '$lib/client/cometmind';
	import type { CometMindStorageSettings } from '$lib/features/settings/schema';
	import {
		nonNegativeIntFromInput,
		positiveIntFromInput
	} from '$lib/features/settings/general-panel-inputs';

	let { storage = $bindable() }: { storage: CometMindStorageSettings } = $props();

	let backupRunning = $state(false);
	let backupMessage = $state('');

	const backupFolderPickerAvailable = $derived(Boolean(window.electronAPI?.selectBackupFolder));

	function patchBackup(patch: Partial<CometMindStorageSettings['backup']>) {
		storage = { ...storage, backup: { ...storage.backup, ...patch } };
	}

	async function chooseBackupFolder() {
		if (!backupFolderPickerAvailable) return;
		const result = await window.electronAPI?.selectBackupFolder?.();
		if (!result || result.canceled || !result.path) return;
		patchBackup({ destinationDir: result.path });
	}

	async function backupNow() {
		if (backupRunning) return;
		if (!storage.backup.destinationDir.trim()) {
			backupMessage = 'Choose a backup folder first.';
			return;
		}
		backupRunning = true;
		backupMessage = '';
		try {
			const result = await runStorageBackup();
			backupMessage = `Backed up ${result.files_zipped} files to ${result.path}`;
		} catch (err) {
			backupMessage = err instanceof Error ? err.message : 'Backup failed';
		} finally {
			backupRunning = false;
		}
	}
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Data backup</h3>
		<p>
			Zip the entire <code>~/.cometmind/</code> directory (wiki, database, settings, skills, OAuth
			tokens) to a folder on your computer.
		</p>
	</div>
	<p class="settings-field-hint backup-warning">
		Backups contain API keys and OAuth tokens. Store them in a safe location.
	</p>

	<SettingsToggle
		label="Enable automatic backup"
		description="Periodically zip ~/.cometmind to the backup folder below."
		checked={storage.backup.enabled}
		onchange={(enabled) => patchBackup({ enabled })}
	/>

	<label class="field backup-folder-field">
		<span>Backup folder</span>
		<div class="backup-folder-input-row">
			<input
				type="text"
				readonly
				aria-readonly="true"
				class="backup-folder-input"
				value={storage.backup.destinationDir || ''}
				placeholder="Choose a backup folder…"
				disabled={!backupFolderPickerAvailable}
				onclick={() => void chooseBackupFolder()}
			/>
			<button
				class="secondary"
				type="button"
				disabled={!backupFolderPickerAvailable}
				onclick={chooseBackupFolder}
			>
				Choose folder…
			</button>
		</div>
		<small>
			{#if storage.backup.destinationDir}
				Archives save as <code>cometmind-backup-*.zip</code> in this folder.
			{:else}
				Not set — choose a folder to enable backups.
			{/if}
		</small>
	</label>

	<label class="field">
		<span>Backup interval (hours)</span>
		<input
			type="number"
			min="1"
			step="1"
			value={storage.backup.intervalHours}
			oninput={(e) => patchBackup({ intervalHours: positiveIntFromInput(e) })}
		/>
	</label>

	<label class="field">
		<span>Keep backups</span>
		<input
			type="number"
			min="0"
			step="1"
			value={storage.backup.maxBackups}
			oninput={(e) => patchBackup({ maxBackups: nonNegativeIntFromInput(e) })}
		/>
		<small>
			{#if storage.backup.maxBackups === 0}
				Keep all backup zip files (no rotation).
			{:else}
				Retain the {storage.backup.maxBackups} most recent archives; delete older ones.
			{/if}
		</small>
	</label>

	<div class="settings-row-actions">
		<button class="secondary" type="button" disabled={backupRunning} onclick={backupNow}>
			{backupRunning ? 'Backing up…' : 'Backup now'}
		</button>
	</div>
	{#if backupMessage}
		<p class="backup-message">{backupMessage}</p>
	{/if}
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 13px;
		color: var(--text-main);
	}

	.field input {
		max-width: 160px;
	}

	.field small {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.backup-warning {
		color: var(--text-muted);
	}

	.backup-folder-field input {
		max-width: none;
	}

	.backup-folder-input-row {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.backup-folder-input-row input {
		flex: 1;
		min-width: 0;
	}

	.backup-folder-input:not(:disabled) {
		cursor: pointer;
	}

	.backup-folder-input:disabled {
		cursor: not-allowed;
	}

	.backup-message {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
