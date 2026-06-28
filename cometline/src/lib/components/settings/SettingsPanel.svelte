<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import {
		Download,
		FolderOpen,
		Keyboard,
		LoaderCircle,
		Palette,
		Power,
		RefreshCw,
		Settings,
		Trash2,
		Upload,
		Workflow,
		X,
		Brain,
		Sparkles
	} from '@lucide/svelte';
	import type { ProviderSettings } from '$lib/types';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import SettingsAppearancePanel from './SettingsAppearancePanel.svelte';
	import SettingsGeneralPanel from './SettingsGeneralPanel.svelte';
	import SettingsCometMindPanel from './SettingsCometMindPanel.svelte';
	import SettingsModelRolesPanel from './SettingsModelRolesPanel.svelte';
	import SettingsMemoryPanel from './SettingsMemoryPanel.svelte';
	import SettingsShortcutsPanel from './SettingsShortcutsPanel.svelte';
	import SettingsProvidersPanel from './SettingsProvidersPanel.svelte';
	import SettingsButton from './SettingsButton.svelte';
	import SettingsTabPersistence from './SettingsTabPersistence.svelte';
	import { ICON_VARIANT_OPTIONS, projectAvatarSrc } from '$lib/project-icon';
	import { heroComposerCssVars } from '$lib/hero-composer-appearance';
	import { onMount } from 'svelte';
	import { createSettingsController } from './settings-controller.svelte';
	import { createSettingsPanelController } from './settings-panel-controller.svelte';
	import { cloneSettings } from '$lib/settings/settings-draft';

	let draft = $state<ProviderSettings>(cloneSettings(settingsStore.settings));
	let selectedProviderId = $state<string>(settingsStore.settings.providers[0]?.id || '');
	let modelSearch = $state('');
	let cometmindPanel = $state<SettingsCometMindPanel | undefined>();
	let memoryPanel = $state<SettingsMemoryPanel | undefined>();

	let selectedProvider = $derived(
		draft.providers.find((p) => p.id === selectedProviderId) ?? draft.providers[0]
	);

	const settingsController = createSettingsController({
		getDraft: () => draft,
		getMemoryPanelDirty: () => memoryPanel?.isDirty?.() ?? false,
		getMemoryPanelBusy: () => memoryPanel?.isBusy?.() ?? false
	});

	const panelController = createSettingsPanelController({
		getDraft: () => draft,
		setDraft: (next) => {
			draft = next;
		},
		getSelectedProviderId: () => selectedProviderId,
		setSelectedProviderId: (id) => {
			selectedProviderId = id;
		},
		getModelSearch: () => modelSearch,
		setModelSearch: (search) => {
			modelSearch = search;
		},
		getSelectedProvider: () => selectedProvider,
		getCometmindPanel: () => cometmindPanel,
		getMemoryPanel: () => memoryPanel,
		settingsController
	});

	let filteredModels = $derived.by(() => {
		if (!selectedProvider) return [];
		const query = modelSearch.trim().toLowerCase();
		if (!query) return selectedProvider.models;
		return selectedProvider.models.filter((model) => model.toLowerCase().includes(query));
	});

	let enabledProviderCount = $derived(
		draft.providers.filter((provider) => provider.enabled).length
	);
	let enabledModelCount = $derived(
		draft.providers.reduce(
			(total, provider) => total + (provider.enabled ? provider.enabledModels.length : 0),
			0
		)
	);

	let modelsSectionWarning = $derived(
		settingsController.activeSection === 'models' &&
			settingsController.hasPendingChanges &&
			enabledModelCount === 0
			? 'Enable at least one model to send messages.'
			: ''
	);

	$effect(() => {
		const vars = heroComposerCssVars(draft.appearance.heroComposer);
		const root = document.documentElement;
		for (const [key, value] of Object.entries(vars)) {
			root.style.setProperty(key, value);
		}
		return () => {
			const saved = heroComposerCssVars(settingsStore.settings.appearance.heroComposer);
			for (const [key, value] of Object.entries(saved)) {
				root.style.setProperty(key, value);
			}
		};
	});

	onMount(() => panelController.initElectron());
</script>

<div class="settings-layer" transition:fade={{ duration: 120 }}>
	<button class="scrim" aria-label="Close settings" onclick={shellStore.closeSettings}></button>
	<div
		class="modal settings-ui"
		role="dialog"
		aria-modal="true"
		aria-labelledby="settings-title"
		transition:scale={{ start: 0.97, duration: 140 }}
	>
		<header>
			<div class="title-mark"><Settings size={16} /></div>
			<div>
				<h2 id="settings-title">Settings</h2>
				<p>
					{#if settingsController.activeSection === 'models'}
						Enable providers, fetch models, and pick which models power each role.
					{:else if settingsController.activeSection === 'appearance'}
						Customize hero composer glow, caret trail, and the project icon.
					{:else if settingsController.activeSection === 'agent'}
						Configure the runtime, OpenCode subagents, skills, and the Discord gateway.
					{:else if settingsController.activeSection === 'memory'}
						Manage global memories, retrieval thresholds, and compaction.
					{:else if settingsController.activeSection === 'shortcuts'}
						Customize keyboard shortcuts.
					{:else}
						Startup, storage, updates, and workspace.
					{/if}
				</p>
			</div>
			<button
				class="icon-button"
				aria-label="Close settings"
				onclick={shellStore.closeSettings}
			>
				<X size={16} />
			</button>
		</header>

		<div class="settings-body">
			<nav class="settings-nav" aria-label="Settings sections">
				<button
					class="settings-nav-item"
					class:selected={settingsController.activeSection === 'models'}
					class:has-pending={settingsController.navSectionDirty('models')}
					onclick={() => panelController.selectSection('models')}
				>
					<Settings size={15} />
					<span>Models</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={settingsController.activeSection === 'memory'}
					class:has-pending={settingsController.navSectionDirty('memory')}
					onclick={() => panelController.selectSection('memory')}
				>
					<Brain size={15} />
					<span>Memory</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={settingsController.activeSection === 'agent'}
					class:has-pending={settingsController.navSectionDirty('agent')}
					onclick={() => panelController.selectSection('agent')}
				>
					<Workflow size={15} />
					<span>Agent</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={settingsController.activeSection === 'appearance'}
					class:has-pending={settingsController.navSectionDirty('appearance')}
					onclick={() => panelController.selectSection('appearance')}
				>
					<Palette size={15} />
					<span>Appearance</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={settingsController.activeSection === 'shortcuts'}
					onclick={() => panelController.selectSection('shortcuts')}
				>
					<Keyboard size={15} />
					<span>Shortcuts</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={settingsController.activeSection === 'app'}
					class:has-pending={settingsController.navSectionDirty('app')}
					onclick={() => panelController.selectSection('app')}
				>
					<Power size={15} />
					<span>App</span>
				</button>
			</nav>

			<div class="settings-pane scrollbar-none">
				{#if settingsController.activeSection === 'models'}
					<div class="settings-panel-stack">
						<SettingsTabPersistence section="models" />
						<SettingsProvidersPanel
							providers={draft.providers}
							bind:selectedProviderId
							bind:modelSearch
							{enabledProviderCount}
							{filteredModels}
							{selectedProvider}
							codexAuthStatus={panelController.codexAuthStatus}
							checkingCodexAuth={panelController.checkingCodexAuth}
							startingCodexLogin={panelController.startingCodexLogin}
							onAddProvider={panelController.addProvider}
							onRemoveProvider={panelController.removeProvider}
							onToggleProvider={panelController.toggleProvider}
							onUpdateSelected={panelController.updateSelected}
							onSetMethod={panelController.setSelectedMethod}
							onFetchModels={panelController.fetchModels}
							onToggleModel={panelController.toggleModel}
							onStartCodexLogin={panelController.startCodexLogin}
							onRefreshCodexAuth={panelController.refreshCodexAuthStatus}
						/>
						<SettingsModelRolesPanel
							bind:cometmind={draft.cometmind}
							bind:defaultModelId={draft.defaultModelId}
							bind:defaultProviderId={draft.defaultProviderId}
							providers={draft.providers}
						/>
					</div>
				{:else if settingsController.activeSection === 'memory'}
					<SettingsTabPersistence section="memory" />
					{#key panelController.memoryPanelKey}
						<SettingsMemoryPanel
							bind:this={memoryPanel}
							providers={draft.providers}
							savedEmbedding={draft.cometmind.memory.embedding}
							onEmbeddingSaved={panelController.persistMemoryEmbedding}
						/>
					{/key}
				{:else if settingsController.activeSection === 'agent'}
					<SettingsTabPersistence section="agent" />
					{#key panelController.cometmindPanelKey}
						<SettingsCometMindPanel
							bind:this={cometmindPanel}
							bind:cometmind={draft.cometmind}
							providers={draft.providers}
							onPickWorkspace={panelController.pickGatewayWorkspace}
						/>
					{/key}
				{:else if settingsController.activeSection === 'appearance'}
					<div class="settings-panel-stack">
						<SettingsTabPersistence section="appearance" />
						<SettingsAppearancePanel
							bind:appearance={draft.appearance.heroComposer}
							bind:caretTrail={draft.appearance.caretTrail}
						/>
						<section class="settings-panel-frame">
							<div class="settings-section">
								<div class="settings-section-heading">
									<div>
										<h3>Project icon</h3>
										<p>
											Chat avatar, intro animation, Dock, menu bar, and SOUL
											persona
										</p>
									</div>
								</div>
								<div
									class="icon-variant-options"
									role="radiogroup"
									aria-label="Project icon style"
								>
									{#each ICON_VARIANT_OPTIONS as option (option.id)}
										<button
											type="button"
											class="icon-variant-chip"
											class:selected={draft.app.iconVariant === option.id}
											role="radio"
											aria-checked={draft.app.iconVariant === option.id}
											onclick={() => panelController.setIconVariant(option.id)}
										>
											<img
												src={projectAvatarSrc(option.id, 96)}
												alt=""
												width="40"
												height="40"
											/>
											<span>{option.label}</span>
										</button>
									{/each}
								</div>
							</div>
						</section>
					</div>
				{:else if settingsController.activeSection === 'shortcuts'}
					<SettingsTabPersistence section="shortcuts" />
					<SettingsShortcutsPanel
						shortcuts={draft.shortcuts}
						onChange={panelController.updateShortcut}
					/>
				{:else}
					<SettingsTabPersistence section="app" />
					<div class="settings-panel-stack">
						<SettingsGeneralPanel
							bind:openAtLogin={draft.app.openAtLogin}
							bind:miniWindowInactivityTimeoutMinutes={draft.app.miniWindowInactivityTimeoutMinutes}
							bind:storage={draft.cometmind.storage}
							onOpenAtLoginChange={panelController.setOpenAtLogin}
						/>
						<section class="settings-panel-frame">
							<div class="settings-panel-body">
								<div class="settings-section">
									<div class="settings-section-heading">
										<div>
											<h3>Settings backup</h3>
											<p>
												Export or import all Cometline settings. Exports may
												include provider API keys.
											</p>
										</div>
									</div>
									<div class="settings-row-actions mb-1">
										<button
											class="secondary"
											onclick={panelController.exportSettings}
											disabled={panelController.exportingSettings}
										>
											{#if panelController.exportingSettings}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{:else}
												<Download size={14} />
											{/if}
											Export
										</button>
										<button
											class="secondary"
											onclick={panelController.importSettings}
											disabled={panelController.importingSettings}
										>
											{#if panelController.importingSettings}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{:else}
												<Upload size={14} />
											{/if}
											Import
										</button>
									</div>
								</div>
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
										<button class="secondary" onclick={panelController.changeWorkspace}>
											<FolderOpen size={14} />
											Change
										</button>
									</div>
								</div>

								<div class="settings-row align-start">
									<div class="settings-row-copy">
										<span class="settings-row-label">Workspace cleanup</span>
										<span class="settings-row-hint">
											Remove deleted workspace folders from /change and
											CometMind registrations.
										</span>
										{#if panelController.workspacePruneMessage}
											<span class="workspace-prune-message"
												>{panelController.workspacePruneMessage}</span
											>
										{/if}
									</div>
									<div class="settings-row-actions">
										<button
											class="secondary"
											onclick={panelController.cleanupWorkspaces}
											disabled={panelController.workspacePruning}
										>
											{#if panelController.workspacePruning}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
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
											class:update-error={panelController.updateState.status === 'error'}
											class:update-ready={panelController.updateState.status === 'ready'}
										>
											{#if panelController.checkingUpdates || panelController.updateState.status === 'checking' || panelController.updateState.status === 'downloading'}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{/if}
											{panelController.updateStatusText}
										</span>
									</div>
									<div class="settings-row-actions">
										{#if panelController.updateState.status === 'ready'}
											<button
												class="primary"
												onclick={panelController.installUpdate}
												disabled={panelController.installingUpdate}
											>
												{#if panelController.installingUpdate}<span class="spin"
														><LoaderCircle size={14} /></span
													>{:else}<Download size={14} />{/if}
												Install update
											</button>
										{:else}
											<button
												class="secondary"
												onclick={panelController.checkForUpdates}
												disabled={!panelController.canCheckUpdates}
											>
												{#if panelController.checkingUpdates || panelController.updateState.status === 'checking'}<span
														class="spin"
														><LoaderCircle size={14} /></span
													>{:else}<RefreshCw size={14} />{/if}
												Check for updates
											</button>
										{/if}
									</div>
								</div>

								<div class="settings-row">
									<div class="settings-row-copy">
										<span class="settings-row-label">Intro</span>
										<span class="settings-row-hint"
											>Replay the first-run animation</span
										>
									</div>
									<div class="settings-row-actions">
										<button class="secondary" onclick={panelController.replayIntro}>
											<Sparkles size={14} />
											Replay intro
										</button>
									</div>
								</div>
								<div class="settings-row">
									<div class="settings-row-copy">
										<span class="settings-row-label">Setup wizard</span>
										<span class="settings-row-hint"
											>Guided provider and model configuration</span
										>
									</div>
									<div class="settings-row-actions">
										<button class="secondary" onclick={panelController.runSetupWizard}>
											<Sparkles size={14} />
											Run setup wizard
										</button>
									</div>
								</div>
								<div class="settings-row">
									<span class="settings-row-label">Version</span>
									<span class="settings-row-value mr-2">{panelController.appVersion || '—'}</span>
								</div>
							</div>
						</section>
					</div>
				{/if}
			</div>
		</div>

		{#if settingsStore.error}
			<p class="message error">{settingsStore.error}</p>
		{:else if settingsController.status}
			<p class="message success">{settingsController.status}</p>
		{/if}

		<footer>
			<p class="settings-footer-copy">
				{#if settingsStore.isSaving}
					Saving changes…
				{:else}
					{#if settingsController.hasPendingChanges}<strong>Unsaved changes ·</strong>{/if}
					Save applies all tabs. Close without saving discards pending edits.
				{/if}
				{#if modelsSectionWarning}<span class="settings-footer-warning"
						>{modelsSectionWarning}</span
					>{/if}
			</p>
			<SettingsButton variant="secondary" onclick={panelController.discardSettings}>Discard</SettingsButton>
			<SettingsButton variant="primary" onclick={panelController.save} disabled={settingsController.saveDisabled}>
				{#if settingsStore.isSaving}<span class="spin"><LoaderCircle size={14} /></span
					>{/if}
				Save changes
			</SettingsButton>
		</footer>
	</div>
</div>

<style>
	.settings-layer {
		position: fixed;
		inset: 0;
		z-index: 80;
		display: grid;
		place-items: center;
		padding: 30px;
	}

	.scrim {
		position: absolute;
		inset: 0;
		border: none;
		background: rgba(17, 24, 39, 0.18);
		backdrop-filter: blur(12px);
	}

	.modal {
		position: relative;
		display: flex;
		flex-direction: column;
		width: min(980px, 100%);
		height: min(760px, calc(100vh - 60px));
		max-height: min(760px, calc(100vh - 60px));
		overflow: hidden;
		background: rgba(255, 255, 255, 0.96);
		border: 1px solid rgba(229, 231, 235, 0.95);
		border-radius: 22px;
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		padding: 18px;
	}

	header,
	footer {
		display: flex;
		align-items: center;
	}

	header {
		position: sticky;
		top: 0;
		z-index: 2;
		flex-shrink: 0;
		gap: 12px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.96);
	}

	.title-mark {
		width: 32px;
		height: 32px;
		border-radius: 11px;
		background: rgba(0, 102, 204, 0.09);
		color: var(--accent);
		display: grid;
		place-items: center;
	}

	header h2,
	header p,
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

	header h2 {
		font-size: 17px;
		font-weight: 700;
	}

	header p,
	footer p {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	header p {
		min-height: calc(1.45em * 2);
	}

	.icon-button {
		margin-left: auto;
		width: 30px;
		height: 30px;
		border: none;
		border-radius: 9px;
		background: transparent;
		color: var(--text-muted);
		display: grid;
		place-items: center;
		cursor: pointer;
	}

	.settings-body {
		display: grid;
		grid-template-columns: 168px 1fr;
		gap: 16px;
		flex: 1;
		min-height: 0;
		overflow: hidden;
		padding: 16px 0;
	}

	.settings-nav {
		display: grid;
		gap: 8px;
		align-content: start;
	}

	.settings-nav-item {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.72);
		padding: 10px 12px;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
		text-align: left;
		cursor: pointer;
	}

	.settings-nav-item.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.settings-pane {
		min-width: 0;
		min-height: 0;
		overflow-y: auto;
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
		color: #027a48;
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

	.icon-variant-options {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
	}

	.icon-variant-chip {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(255, 255, 255, 0.76);
		padding: 8px 12px 8px 8px;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.icon-variant-chip img {
		width: 40px;
		height: 40px;
		border-radius: 999px;
		object-fit: cover;
		border: 1px solid rgba(15, 23, 42, 0.08);
	}

	.icon-variant-chip.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.icon-variant-chip:hover {
		background: rgba(15, 23, 42, 0.08);
	}

	@media (max-width: 780px) {
		.settings-body {
			grid-template-columns: 1fr;
		}

		.modal {
			height: calc(100vh - 40px);
			max-height: calc(100vh - 40px);
		}
	}
</style>
