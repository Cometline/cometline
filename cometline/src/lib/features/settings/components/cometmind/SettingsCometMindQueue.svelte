<script lang="ts">
	import { FolderOpen } from '@lucide/svelte';
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { CometMindSettings } from '$lib/cometmind-settings';
	import type { ProviderConfig } from '$lib/types';

	let {
		cometmind = $bindable(),
		allowedUsersText = $bindable(),
		allowedChannelsText = $bindable(),
		providers,
		discordProvider,
		discordModels,
		gatewayRunning,
		gatewayBusy,
		onPickWorkspace,
		setDiscordProvider,
		setDiscordModel,
		onDiscordGatewayToggle,
		syncListsFromText,
		useCurrentWorkspace
	}: {
		cometmind: CometMindSettings;
		allowedUsersText: string;
		allowedChannelsText: string;
		providers: ProviderConfig[];
		discordProvider: ProviderConfig | undefined;
		discordModels: string[];
		gatewayRunning: boolean;
		gatewayBusy: boolean;
		onPickWorkspace?: () => void | Promise<void>;
		setDiscordProvider: (providerId: string) => void;
		setDiscordModel: (modelId: string) => void;
		onDiscordGatewayToggle: (enabled: boolean) => void;
		syncListsFromText: () => void;
		useCurrentWorkspace: () => void;
	} = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Jobs</h3>
		<p>Global work queue notifications and lease timing.</p>
	</div>
	<SettingsToggle
		label="Job notifications"
		description="Show desktop alerts when jobs are claimed or completed."
		bind:checked={cometmind.jobs.notifications.enabled}
	/>
	<SettingsToggle
		label="Notify on claimed"
		description="Alert when a job is claimed by a session."
		bind:checked={cometmind.jobs.notifications.onClaimed}
		disabled={!cometmind.jobs.notifications.enabled}
	/>
	<SettingsToggle
		label="Notify on completed"
		description="Alert when a job is marked done."
		bind:checked={cometmind.jobs.notifications.onCompleted}
		disabled={!cometmind.jobs.notifications.enabled}
	/>
	<SettingsToggle
		label="Notify on released"
		description="Alert when an ongoing job returns to todo."
		bind:checked={cometmind.jobs.notifications.onReleased}
		disabled={!cometmind.jobs.notifications.enabled}
	/>
	<SettingsToggle
		label="Notify on blocked"
		description="Alert when a job is blocked after repeated failures."
		bind:checked={cometmind.jobs.notifications.onBlocked}
		disabled={!cometmind.jobs.notifications.enabled}
	/>
	<label>
		<span>Lease duration (minutes)</span>
		<input type="number" min="1" step="1" bind:value={cometmind.jobs.leaseMinutes} />
	</label>
	<label>
		<span>Reconcile interval (seconds)</span>
		<input
			type="number"
			min="30"
			step="1"
			bind:value={cometmind.jobs.reconcileIntervalSeconds}
		/>
	</label>
	<label>
		<span>Deleted job purge (days)</span>
		<input type="number" min="0" step="1" bind:value={cometmind.jobs.deletedPurgeDays} />
		<p class="settings-field-hint">Set to 0 to keep soft-deleted jobs.</p>
	</label>
	<label>
		<span>Auto-archive completed jobs (days)</span>
		<input type="number" min="0" step="1" bind:value={cometmind.jobs.doneArchiveDays} />
		<p class="settings-field-hint">
			Moves completed jobs out of the active Done column after this many days. Set to 0 to
			keep completed jobs visible.
		</p>
	</label>
	<label>
		<span>Archived job purge (days)</span>
		<input type="number" min="0" step="1" bind:value={cometmind.jobs.archivedPurgeDays} />
		<p class="settings-field-hint">
			Hard-deletes archived jobs older than this many days. Set to 0 to keep archived jobs
			indefinitely.
		</p>
	</label>
	<label>
		<span>Stale ongoing review (minutes)</span>
		<input type="number" min="1" step="1" bind:value={cometmind.jobs.staleReviewMinutes} />
		<p class="settings-field-hint">
			Jobs stuck in ongoing longer than this are logged as stale during maintenance.
		</p>
	</label>
	<label>
		<span>Max consecutive failures</span>
		<input type="number" min="1" step="1" bind:value={cometmind.jobs.maxConsecutiveFailures} />
	</label>
	<label>
		<span>Retry cooldown (minutes)</span>
		<input type="number" min="1" step="1" bind:value={cometmind.jobs.retryCooldownMinutes} />
	</label>
	<label>
		<span>Max retry cooldown (minutes)</span>
		<input type="number" min="1" step="1" bind:value={cometmind.jobs.maxRetryCooldownMinutes} />
	</label>
</div>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Autonomous jobs</h3>
		<p>
			Let CometMind claim and run ready jobs on its own, without a human opening a chat
			session first. Off by default.
		</p>
	</div>
	<SettingsToggle
		label="Enable autonomous job pickup"
		description="A background worker polls the job queue and executes ready jobs automatically."
		bind:checked={cometmind.autonomy.enabled}
	/>
	<label>
		<span>Max concurrent jobs</span>
		<input
			type="number"
			min="1"
			step="1"
			bind:value={cometmind.autonomy.maxConcurrent}
			disabled={!cometmind.autonomy.enabled}
		/>
	</label>
	<label>
		<span>Poll interval (seconds)</span>
		<input
			type="number"
			min="5"
			step="1"
			bind:value={cometmind.autonomy.pollIntervalSeconds}
			disabled={!cometmind.autonomy.enabled}
		/>
	</label>
</div>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Scheduler</h3>
		<p>Materialize deferred and recurring scheduled jobs into the job queue. Off by default.</p>
	</div>
	<SettingsToggle
		label="Enable scheduler"
		description="A background ticker polls for due scheduled jobs and creates normal job entries."
		bind:checked={cometmind.scheduler.enabled}
	/>
	<label>
		<span>Poll interval (seconds)</span>
		<input
			type="number"
			min="10"
			step="1"
			bind:value={cometmind.scheduler.pollIntervalSeconds}
			disabled={!cometmind.scheduler.enabled}
		/>
	</label>
</div>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Discord gateway</h3>
		<p>
			Runs <code>cometmind gateway run --platform discord</code> while Cometline is open.
			Settings are saved to <code>~/.cometmind/cometline-settings.json</code>.
		</p>
	</div>
	<div class="gateway-runtime">
		<SettingsToggle
			label="Run Discord gateway"
			description="Start the Discord bot automatically while this app is running."
			bind:checked={cometmind.gateway.discord.enabled}
			disabled={gatewayBusy || !window.electronAPI?.setDiscordGatewayEnabled}
			onchange={onDiscordGatewayToggle}
		/>
		<p class="gateway-status" class:running={gatewayRunning}>
			Status: {gatewayRunning ? 'Running' : 'Stopped'}
		</p>
	</div>
	<label>
		<span>Bot Token</span>
		<input
			type="password"
			bind:value={cometmind.gateway.discord.botToken}
			placeholder="Paste from Discord Developer Portal"
			spellcheck="false"
			autocomplete="off"
		/>
	</label>
	<label>
		<span>Default provider</span>
		<select
			value={cometmind.gateway.discord.providerId || discordProvider?.id || ''}
			onchange={(e) => setDiscordProvider(e.currentTarget.value)}
		>
			{#each providers as provider (provider.id)}
				<option value={provider.id}>{provider.name}</option>
			{/each}
		</select>
	</label>
	<label>
		<span>Default model</span>
		<select
			value={cometmind.gateway.discord.modelId || discordModels[0] || ''}
			onchange={(e) => setDiscordModel(e.currentTarget.value)}
		>
			{#each discordModels as model (model)}
				<option value={model}>{model}</option>
			{/each}
		</select>
		<p class="settings-field-hint">
			Used for new Discord / thread sessions. Falls back to the global CometMind model when
			empty.
		</p>
	</label>
	<label>
		<span>Workspace path (repo for the gateway)</span>
		<div class="path-row">
			<input
				type="text"
				bind:value={cometmind.gateway.discord.workspacePath}
				placeholder="/path/to/cometline-release"
				spellcheck="false"
			/>
			<button class="secondary" type="button" onclick={useCurrentWorkspace}
				>Current workspace</button
			>
			{#if onPickWorkspace}
				<button
					class="secondary icon"
					type="button"
					aria-label="Choose folder"
					onclick={onPickWorkspace}
				>
					<FolderOpen size={14} />
				</button>
			{/if}
		</div>
	</label>
	<label>
		<span>Allowed user IDs (one per line)</span>
		<textarea
			bind:value={allowedUsersText}
			oninput={syncListsFromText}
			onchange={syncListsFromText}
			onblur={syncListsFromText}
			rows="3"
			placeholder="123456789012345678"
			spellcheck="false"
		></textarea>
	</label>
	<label>
		<span>Allowed channel IDs (one per line; leave empty for no channel restriction)</span>
		<textarea
			bind:value={allowedChannelsText}
			oninput={syncListsFromText}
			onchange={syncListsFromText}
			onblur={syncListsFromText}
			rows="3"
			placeholder="987654321098765432"
			spellcheck="false"
		></textarea>
	</label>
	<label class="checkbox-row">
		<input type="checkbox" bind:checked={cometmind.gateway.discord.requireMention} />
		<span>Require @mention in server channels</span>
	</label>
</div>

<style>
	.gateway-runtime {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.gateway-status {
		margin: 0;
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.gateway-status.running {
		color: var(--color-2f6f4f);
	}

	textarea {
		resize: vertical;
		min-height: 72px;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 12px;
	}

	.checkbox-row {
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}

	.path-row {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.path-row input {
		flex: 1;
		min-width: 0;
	}
</style>
