<script lang="ts">
	import {
		Download,
		FolderOpen,
		LoaderCircle,
		RefreshCw,
		Sparkles,
		Trash2
	} from '@lucide/svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import type { createSettingsPanelController } from '../../settings-panel-controller.svelte';

	let { controller }: { controller: ReturnType<typeof createSettingsPanelController> } = $props();
</script>

<section class="settings-panel-frame">
	<div class="settings-panel-body">
		<div class="settings-row align-start">
			<div class="settings-row-copy">
				<span class="settings-row-label">Workspace</span>
				<span
					class="settings-row-value workspace-path"
					title={shellStore.defaultWorkspacePath}
				>
					{shellStore.defaultWorkspacePath}
				</span>
			</div>
			<div class="settings-row-actions">
				<button class="secondary" onclick={controller.changeWorkspace}>
					<FolderOpen size={14} />
					Change
				</button>
			</div>
		</div>

		<div class="settings-row align-start">
			<div class="settings-row-copy">
				<span class="settings-row-label">Workspace cleanup</span>
				<span class="settings-row-hint">
					Remove deleted workspace folders from /change and CometMind registrations.
				</span>
				{#if controller.workspacePruneMessage}
					<span class="workspace-prune-message">{controller.workspacePruneMessage}</span>
				{/if}
			</div>
			<div class="settings-row-actions">
				<button
					class="secondary"
					onclick={controller.cleanupWorkspaces}
					disabled={controller.workspacePruning}
				>
					{#if controller.workspacePruning}
						<span class="spin small"><LoaderCircle size={14} /></span>
					{:else}
						<Trash2 size={14} />
					{/if}
					Clean up
				</button>
			</div>
		</div>

		<div class="settings-row align-start">
			<div class="settings-row-copy">
				<span class="settings-row-label">Updates</span>
				<span
					class="update-status"
					class:update-error={controller.updateState.status === 'error'}
					class:update-ready={controller.updateState.status === 'ready'}
				>
					{#if controller.checkingUpdates || controller.updateState.status === 'checking' || controller.updateState.status === 'downloading'}
						<span class="spin small"><LoaderCircle size={14} /></span>
					{/if}
					{controller.updateStatusText}
				</span>
			</div>
			<div class="settings-row-actions">
				{#if controller.updateState.status === 'ready'}
					<button
						class="primary"
						onclick={controller.installUpdate}
						disabled={controller.installingUpdate}
					>
						{#if controller.installingUpdate}<span class="spin"
								><LoaderCircle size={14} /></span
							>{:else}<Download size={14} />{/if}
						Install update
					</button>
				{:else}
					<button
						class="secondary"
						onclick={controller.checkForUpdates}
						disabled={!controller.canCheckUpdates}
					>
						{#if controller.checkingUpdates || controller.updateState.status === 'checking'}<span
								class="spin"><LoaderCircle size={14} /></span
							>{:else}<RefreshCw size={14} />{/if}
						Check for updates
					</button>
				{/if}
			</div>
		</div>

		<div class="settings-row">
			<div class="settings-row-copy">
				<span class="settings-row-label">Intro</span>
				<span class="settings-row-hint">Replay the first-run animation</span>
			</div>
			<div class="settings-row-actions">
				<button class="secondary" onclick={controller.replayIntro}>
					<Sparkles size={14} />
					Replay intro
				</button>
			</div>
		</div>
		<div class="settings-row">
			<div class="settings-row-copy">
				<span class="settings-row-label">Setup wizard</span>
				<span class="settings-row-hint">Guided provider and model configuration</span>
			</div>
			<div class="settings-row-actions">
				<button class="secondary" onclick={controller.runSetupWizard}>
					<Sparkles size={14} />
					Run setup wizard
				</button>
			</div>
		</div>
		<div class="settings-row">
			<span class="settings-row-label">Version</span>
			<span class="settings-row-value mr-2">{controller.appVersion || '—'}</span>
		</div>
	</div>
</section>

<style>
	.update-status {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.update-status.update-error {
		color: var(--status-error);
	}

	.update-status.update-ready {
		color: var(--color-027a48);
	}

	.workspace-path {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 420px;
	}

	.workspace-prune-message {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
		max-width: 420px;
	}
</style>
