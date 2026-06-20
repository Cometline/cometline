<script lang="ts">
	import SettingsToggle from './SettingsToggle.svelte';
	import type { CometMindStorageSettings } from '$lib/settings/schema';

	let {
		openAtLogin = $bindable(false),
		storage = $bindable<CometMindStorageSettings>(),
		onOpenAtLoginChange
	}: {
		openAtLogin: boolean;
		storage: CometMindStorageSettings;
		onOpenAtLoginChange?: (enabled: boolean) => void | Promise<void>;
	} = $props();

	function patchStorage(patch: Partial<CometMindStorageSettings>) {
		storage = { ...storage, ...patch };
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
	</div>

	<div class="settings-section">
		<div class="settings-section-heading">
			<h3>Storage & retention</h3>
			<p>
				Automatic cleanup runs when CometMind starts and after settings are saved. Set a
				field to 0 to disable that rule.
			</p>
		</div>

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

		<SettingsToggle
			label="Vacuum database after purge"
			description="Reclaim disk space in cometmind.db after sessions or memories are deleted."
			checked={storage.vacuumAfterPurge}
			onchange={(enabled) => patchStorage({ vacuumAfterPurge: enabled })}
		/>

		<p class="discord-note">
			Deleting a session also removes its Discord channel mapping. The next message in that
			channel starts a fresh session without prior Cometline history.
		</p>
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
</style>
