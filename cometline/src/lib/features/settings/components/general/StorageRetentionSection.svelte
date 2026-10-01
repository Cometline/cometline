<script lang="ts">
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { CometMindStorageSettings } from '$lib/features/settings/schema';
	import { nonNegativeIntFromInput } from '$lib/features/settings/general-panel-inputs';

	let { storage = $bindable() }: { storage: CometMindStorageSettings } = $props();

	function patchStorage(patch: Partial<CometMindStorageSettings>) {
		storage = { ...storage, ...patch };
	}
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Storage & retention</h3>
		<p>Control how long CometMind keeps archived sessions and memory before purging.</p>
	</div>
	<p class="settings-field-hint">
		Automatic cleanup runs on a CometMind schedule. Set a retention field to 0 to disable that
		rule. Interval changes apply on Save without restarting the sidecar.
	</p>

	<label class="field">
		<span>Cleanup interval (minutes)</span>
		<input
			type="number"
			min="0"
			step="1"
			value={storage.cleanupIntervalMinutes}
			oninput={(e) => patchStorage({ cleanupIntervalMinutes: nonNegativeIntFromInput(e) })}
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
			oninput={(e) => patchStorage({ retentionDays: nonNegativeIntFromInput(e) })}
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
		<span>Media after chat deletion (days)</span>
		<input
			type="number"
			min="0"
			step="1"
			value={storage.detachedMediaRetentionDays}
			oninput={(e) =>
				patchStorage({ detachedMediaRetentionDays: nonNegativeIntFromInput(e) })}
		/>
		<small>
			{#if storage.detachedMediaRetentionDays === 0}
				Disabled — Gallery media stays after its chat is deleted.
			{:else}
				Delete Gallery media {storage.detachedMediaRetentionDays} day{storage.detachedMediaRetentionDays ===
				1
					? ''
					: 's'} after its chat is deleted.
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
			oninput={(e) => patchStorage({ maxSessionsPerWorkspace: nonNegativeIntFromInput(e) })}
		/>
		<small>
			{#if storage.maxSessionsPerWorkspace === 0}
				Disabled — no limit on session count.
			{:else}
				Keep the {storage.maxSessionsPerWorkspace} most recently updated sessions; delete older
				ones.
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
			oninput={(e) => patchStorage({ archivedMemoryPurgeDays: nonNegativeIntFromInput(e) })}
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
		<span>Purge tool-output files (days)</span>
		<input
			type="number"
			min="0"
			step="1"
			value={storage.toolOutputRetentionDays}
			oninput={(e) => patchStorage({ toolOutputRetentionDays: nonNegativeIntFromInput(e) })}
		/>
		<small>
			{#if storage.toolOutputRetentionDays === 0}
				Disabled — spilled tool output under <code>~/.cometmind/tool-output/</code> stays on disk.
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
			oninput={(e) => patchStorage({ agentTmpRetentionDays: nonNegativeIntFromInput(e) })}
		/>
		<small>
			{#if storage.agentTmpRetentionDays === 0}
				Disabled — <code>~/.cometmind/agent-tmp/</code> files stay on disk.
			{:else}
				Delete <code>agent-tmp/</code> files older than {storage.agentTmpRetentionDays}
				days.
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
		Deleting a session also removes its Discord channel mapping. The next message in that
		channel starts a fresh session without prior Cometline history.
	</p>
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
