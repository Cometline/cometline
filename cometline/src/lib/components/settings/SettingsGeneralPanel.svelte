<script lang="ts">
	import SettingsToggle from './SettingsToggle.svelte';
	import SettingsPersistenceHint from './SettingsPersistenceHint.svelte';
	import { runStorageBackup } from '$lib/client/cometmind';
	import type { CometMindStorageSettings } from '$lib/settings/schema';

	let {
		openAtLogin = $bindable(false),
		miniWindowInactivityTimeoutMinutes = $bindable(30),
		storage = $bindable<CometMindStorageSettings>(),
		onOpenAtLoginChange
	}: {
		openAtLogin: boolean;
		miniWindowInactivityTimeoutMinutes: number;
		storage: CometMindStorageSettings;
		onOpenAtLoginChange?: (enabled: boolean) => void | Promise<void>;
	} = $props();

	let backupRunning = $state(false);
	let backupMessage = $state('');

	const backupFolderPickerAvailable = $derived(
		Boolean(window.electronAPI?.selectBackupFolder)
	);

	function onMiniWindowTimeoutInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		miniWindowInactivityTimeoutMinutes = Number.isFinite(value)
			? Math.min(24 * 60, Math.max(1, Math.floor(value)))
			: 30;
	}

	function patchStorage(patch: Partial<CometMindStorageSettings>) {
		storage = { ...storage, ...patch };
	}

	function onCleanupIntervalInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			cleanupIntervalMinutes: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onRetentionDaysInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			retentionDays: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onMaxSessionsInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			maxSessionsPerWorkspace: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onPurgeDaysInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			archivedMemoryPurgeDays: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onDeletedJobPurgeDaysInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			deletedJobPurgeDays: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onToolOutputRetentionInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			toolOutputRetentionDays: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onAgentTmpRetentionInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			agentTmpRetentionDays: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
		});
	}

	function onBackupIntervalInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			backup: {
				...storage.backup,
				intervalHours: Number.isFinite(value) ? Math.max(1, Math.floor(value)) : 1
			}
		});
	}

	function onBackupMaxInput(event: Event) {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		patchStorage({
			backup: {
				...storage.backup,
				maxBackups: Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0
			}
		});
	}

	async function chooseBackupFolder() {
		if (!backupFolderPickerAvailable) return;
		const result = await window.electronAPI?.selectBackupFolder?.();
		if (!result || result.canceled || !result.path) return;
		patchStorage({
			backup: { ...storage.backup, destinationDir: result.path }
		});
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

<section class="general-panel settings-panel-frame">
	<div class="settings-panel-body">
		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Startup</h3>
				<p>Control how Cometline launches on your Mac.</p>
			</div>
			<SettingsToggle
				label="Open at login"
				description="Launch Cometline when you sign in. On macOS 13+, you may need to approve it in System Settings → Login Items."
				bind:checked={openAtLogin}
				disabled={!window.electronAPI?.setOpenAtLogin}
				onchange={onOpenAtLoginChange}
			/>
			<SettingsPersistenceHint tier="instant" />
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Mini window</h3>
				<p>Control when the compact window starts a fresh rolling session.</p>
			</div>
			<SettingsPersistenceHint tier="pending" detail="Included in Save changes" />
			<label class="field">
				<span>Mini window reset timeout (minutes)</span>
				<input
					type="number"
					min="1"
					max="1440"
					step="1"
					value={miniWindowInactivityTimeoutMinutes}
					oninput={onMiniWindowTimeoutInput}
				/>
				<small>
					After the mini window stays hidden long enough, the next hotkey open starts a
					new rolling session. Current setting: {miniWindowInactivityTimeoutMinutes} minute{miniWindowInactivityTimeoutMinutes ===
					1
						? ''
						: 's'}.
				</small>
			</label>
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Storage & retention</h3>
				<p>Control how long CometMind keeps archived sessions and memory before purging.</p>
			</div>
			<SettingsPersistenceHint
				tier="pending"
				detail="Included in Save changes — reloads CometMind in place"
			/>
			<p class="settings-field-hint">
				Automatic cleanup runs on a CometMind schedule. Set a retention field to 0 to
				disable that rule. Interval changes apply on Save without restarting the sidecar.
			</p>

			<label class="field">
				<span>Cleanup interval (minutes)</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.cleanupIntervalMinutes}
					oninput={onCleanupIntervalInput}
				/>
				<small>
					{#if storage.cleanupIntervalMinutes === 0}
						Use the default 60 minute cleanup interval.
					{:else}
						Check cleanup rules every {storage.cleanupIntervalMinutes} minute{storage.cleanupIntervalMinutes ===
						1
							? ''
							: 's'}.
					{/if}
				</small>
			</label>

			<label class="field">
				<span>Session retention (days)</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.retentionDays}
					oninput={onRetentionDaysInput}
				/>
				<small>
					{#if storage.retentionDays === 0}
						Disabled — sessions are not deleted by age.
					{:else}
						Delete sessions with no activity for {storage.retentionDays} days.
					{/if}
				</small>
			</label>

			<label class="field">
				<span>Max sessions per workspace</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.maxSessionsPerWorkspace}
					oninput={onMaxSessionsInput}
				/>
				<small>
					{#if storage.maxSessionsPerWorkspace === 0}
						Disabled — no limit on session count.
					{:else}
						Keep the {storage.maxSessionsPerWorkspace} most recently updated sessions; delete
						older ones.
					{/if}
				</small>
			</label>

			<label class="field">
				<span>Purge archived memories (days)</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.archivedMemoryPurgeDays}
					oninput={onPurgeDaysInput}
				/>
				<small>
					{#if storage.archivedMemoryPurgeDays === 0}
						Disabled — archived memories stay on disk.
					{:else}
						Hard-delete archived memories older than {storage.archivedMemoryPurgeDays} days.
					{/if}
				</small>
			</label>

			<label class="field">
				<span>Purge deleted jobs (days)</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.deletedJobPurgeDays}
					oninput={onDeletedJobPurgeDaysInput}
				/>
				<small>
					{#if storage.deletedJobPurgeDays === 0}
						Disabled — soft-deleted jobs stay on disk.
					{:else}
						Hard-delete soft-deleted jobs older than {storage.deletedJobPurgeDays} days.
					{/if}
				</small>
			</label>

			<label class="field">
				<span>Purge tool-output files (days)</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.toolOutputRetentionDays}
					oninput={onToolOutputRetentionInput}
				/>
				<small>
					{#if storage.toolOutputRetentionDays === 0}
						Disabled — spilled tool output under <code>~/.cometmind/tool-output/</code> stays on
						disk.
					{:else}
						Delete <code>tool-output/</code> files older than {storage.toolOutputRetentionDays}
						days.
					{/if}
				</small>
			</label>

			<label class="field">
				<span>Purge agent-tmp files (days)</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.agentTmpRetentionDays}
					oninput={onAgentTmpRetentionInput}
				/>
				<small>
					{#if storage.agentTmpRetentionDays === 0}
						Disabled — <code>~/.cometmind/agent-tmp/</code> files stay on disk.
					{:else}
						Delete <code>agent-tmp/</code> files older than {storage.agentTmpRetentionDays} days.
					{/if}
				</small>
			</label>

			<SettingsToggle
				label="Vacuum database after purge"
				description="Reclaim disk space in cometmind.db after sessions or memories are deleted."
				checked={storage.vacuumAfterPurge}
				onchange={(enabled) => patchStorage({ vacuumAfterPurge: enabled })}
			/>

			<p class="settings-field-hint">
				Sidecar logs live under <code>~/.cometmind/logs/</code> (10MB rotate).
			</p>

			<p class="discord-note">
				Deleting a session also removes its Discord channel mapping. The next message in
				that channel starts a fresh session without prior Cometline history.
			</p>
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Data backup</h3>
				<p>
					Zip the entire <code>~/.cometmind/</code> directory (wiki, database, settings,
					skills, OAuth tokens) to a folder on your computer.
				</p>
			</div>
			<SettingsPersistenceHint
				tier="pending"
				detail="Backup folder and schedule are included in Save changes"
			/>
			<p class="settings-field-hint backup-warning">
				Backups contain API keys and OAuth tokens. Store them in a safe location.
			</p>

			<SettingsToggle
				label="Enable automatic backup"
				description="Periodically zip ~/.cometmind to the backup folder below."
				checked={storage.backup.enabled}
				onchange={(enabled) =>
					patchStorage({ backup: { ...storage.backup, enabled } })}
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
					oninput={onBackupIntervalInput}
				/>
			</label>

			<label class="field">
				<span>Keep backups</span>
				<input
					type="number"
					min="0"
					step="1"
					value={storage.backup.maxBackups}
					oninput={onBackupMaxInput}
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
	</div>
</section>

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

	.discord-note {
		margin: 4px 0 0;
		padding: 10px 12px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--text-muted) 8%, transparent);
		font-size: 12px;
		line-height: 1.5;
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
