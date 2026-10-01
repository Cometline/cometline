<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import type { ProviderSettings } from '$lib/types';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import SettingsAppearancePanel from './SettingsAppearancePanel.svelte';
	import SettingsGeneralPanel from './SettingsGeneralPanel.svelte';
	import SettingsCometMindPanel from './SettingsCometMindPanel.svelte';
	import SettingsModelRolesPanel from './SettingsModelRolesPanel.svelte';
	import { normalizeModelRoleDraft } from '$lib/features/settings/model-role-draft';
	import SettingsMemoryPanel from './SettingsMemoryPanel.svelte';
	import SettingsShortcutsPanel from './SettingsShortcutsPanel.svelte';
	import SettingsProvidersPanel from './SettingsProvidersPanel.svelte';
	import SettingsTabPersistence from './SettingsTabPersistence.svelte';
	import SettingsPanelHeader from './settings-panel/SettingsPanelHeader.svelte';
	import SettingsPanelNav from './settings-panel/SettingsPanelNav.svelte';
	import SettingsPanelFooter from './settings-panel/SettingsPanelFooter.svelte';
	import SettingsPanelAppSection from './settings-panel/SettingsPanelAppSection.svelte';
	import SettingsPanelPersonaSection from './settings-panel/SettingsPanelPersonaSection.svelte';
	import SettingsPanelPersonaDialogs from './settings-panel/SettingsPanelPersonaDialogs.svelte';
	import { onMount } from 'svelte';
	import { createSettingsController } from '../settings-controller.svelte';
	import { createSettingsPanelController } from '../settings-panel-controller.svelte';
	import { createSettingsPanelPersonaEditor } from '../settings-panel-persona-editor.svelte';
	import { previewDraftHeroComposer } from '../settings-panel-hero-preview.svelte';
	import type { SettingsPanelMode } from '../settings-panel-types';
	import {
		countEnabledModels,
		countEnabledProviders,
		filterProviderModels,
		modelsSectionWarningText
	} from '../settings-panel-models';
	import { cloneSettings } from '$lib/features/settings/settings-draft';

	let { mode = 'modal', onClose }: { mode?: SettingsPanelMode; onClose?: () => void } = $props();

	let draft = $state<ProviderSettings>(
		normalizeModelRoleDraft(cloneSettings(settingsStore.settings))
	);
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
			draft = normalizeModelRoleDraft(next);
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
		closeSettings,
		getMode: () => mode,
		settingsController
	});

	const personaEditor = createSettingsPanelPersonaEditor({
		saveCustomPersona: panelController.saveCustomPersona,
		deleteCustomPersona: panelController.deleteCustomPersona
	});

	onMount(() => {
		void personaEditor.refreshCustomPersonas();
	});

	let filteredModels = $derived(filterProviderModels(selectedProvider, modelSearch));

	function closeSettings() {
		onClose?.();
		if (!onClose) shellStore.closeSettings();
	}

	let enabledProviderCount = $derived(countEnabledProviders(draft.providers));
	let enabledModelCount = $derived(countEnabledModels(draft.providers));

	let modelsSectionWarning = $derived(
		modelsSectionWarningText(
			settingsController.activeSection,
			settingsController.hasPendingChanges,
			enabledModelCount
		)
	);

	previewDraftHeroComposer(() => draft.appearance.heroComposer);

	onMount(() => panelController.initElectron());
</script>

<div
	class="settings-layer"
	class:window-mode={mode === 'window'}
	transition:fade={{ duration: mode === 'modal' ? 120 : 0 }}
>
	{#if mode === 'modal'}
		<button class="scrim" aria-label="Close settings" onclick={closeSettings}></button>
	{/if}
	<div
		class="modal settings-ui"
		class:window-mode={mode === 'window'}
		role={mode === 'modal' ? 'dialog' : undefined}
		aria-modal={mode === 'modal' ? 'true' : undefined}
		aria-labelledby="settings-title"
		transition:scale={{
			start: mode === 'modal' ? 0.97 : 1,
			duration: mode === 'modal' ? 140 : 0
		}}
	>
		<SettingsPanelHeader
			{mode}
			activeSection={settingsController.activeSection}
			onClose={closeSettings}
		/>

		<div class="settings-body">
			<SettingsPanelNav
				activeSection={settingsController.activeSection}
				navSectionDirty={settingsController.navSectionDirty}
				onSelect={panelController.selectSection}
			/>

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
							xaiAuthStatus={panelController.xaiAuthStatus}
							checkingXaiAuth={panelController.checkingXaiAuth}
							startingXaiLogin={panelController.startingXaiLogin}
							onAddProvider={panelController.addProvider}
							onRemoveProvider={panelController.removeProvider}
							onToggleProvider={panelController.toggleProvider}
							onUpdateSelected={panelController.updateSelected}
							onSetMethod={panelController.setSelectedMethod}
							onFetchModels={panelController.fetchModels}
							onToggleModel={panelController.toggleModel}
							onStartCodexLogin={panelController.startCodexLogin}
							onRefreshCodexAuth={panelController.refreshCodexAuthStatus}
							onStartXaiLogin={panelController.startXaiLogin}
							onRefreshXaiAuth={panelController.refreshXaiAuthStatus}
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
							onPersistBeforeRuntimeAction={panelController.persistDraftForRuntime}
						/>
					{/key}
				{:else if settingsController.activeSection === 'appearance'}
					<div class="settings-panel-stack">
						<SettingsTabPersistence section="appearance" />
						<SettingsAppearancePanel
							bind:appearance={draft.appearance.heroComposer}
							bind:caretTrail={draft.appearance.caretTrail}
							bind:terminal={draft.appearance.terminal}
							bind:responseCompleteSound={draft.appearance.responseCompleteSound}
						/>
						<SettingsPanelPersonaSection
							app={draft.app}
							editor={personaEditor}
							onSelectPersona={panelController.setPersonaId}
						/>
					</div>

					<SettingsPanelPersonaDialogs editor={personaEditor} />
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
							bind:screenCapturePreferred={draft.app.screenCapturePreferred}
							screenCaptureStatus={panelController.screenCaptureStatus}
							bind:confirmCloseOnCmdW={draft.app.confirmCloseOnCmdW}
							bind:confirmBeforeDeletingChats={draft.app.confirmBeforeDeletingChats}
							bind:confirmBeforeDeletingMedia={draft.app.confirmBeforeDeletingMedia}
							bind:fileSearchSource={draft.app.fileSearchSource}
							bind:miniWindowInactivityTimeoutMinutes={
								draft.app.miniWindowInactivityTimeoutMinutes
							}
							bind:storage={draft.cometmind.storage}
							onOpenAtLoginChange={panelController.setOpenAtLogin}
							onScreenCapturePreferredChange={panelController.setScreenCapturePreferred}
							onOpenScreenCaptureSettings={panelController.openScreenCaptureSettings}
							onConfirmCloseOnCmdWChange={panelController.setConfirmCloseOnCmdW}
							onConfirmBeforeDeletingChatsChange={panelController.setConfirmBeforeDeletingChats}
							onConfirmBeforeDeletingMediaChange={panelController.setConfirmBeforeDeletingMedia}
							onFileSearchSourceChange={panelController.setFileSearchSource}
						/>
						<SettingsPanelAppSection controller={panelController} />
					</div>
				{/if}
			</div>
		</div>

		<SettingsPanelFooter
			status={settingsController.status}
			hasPendingChanges={settingsController.hasPendingChanges}
			{modelsSectionWarning}
			saveDisabled={settingsController.saveDisabled}
			onDiscard={panelController.discardSettings}
			onSave={panelController.save}
		/>
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

	.settings-layer.window-mode {
		position: relative;
		min-height: 100vh;
		padding: 0;
		background: rgba(255, 255, 255, 0.96);
	}

	.settings-layer.window-mode::before {
		content: '';
		position: absolute;
		inset: 0 0 auto;
		height: 46px;
		z-index: 3;
		-webkit-app-region: drag;
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

	.modal.window-mode {
		width: 100%;
		height: 100vh;
		max-height: none;
		background: rgba(255, 255, 255, 0.96);
		border: none;
		border-radius: 0;
		box-shadow: none;
		padding: 0 28px 16px;
		-webkit-app-region: no-drag;
	}

	.modal.window-mode .settings-body {
		display: flex;
		flex-direction: column;
		gap: 0;
		padding: 6px 0 18px;
	}

	.modal.window-mode .settings-pane {
		width: min(100%, 1160px);
		margin: 0 auto;
		padding: 18px 2px 0;
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

	.settings-pane {
		min-width: 0;
		min-height: 0;
		overflow-y: auto;
	}

	@media (max-width: 780px) {
		.settings-body {
			grid-template-columns: 1fr;
		}

		.modal {
			height: calc(100vh - 40px);
			max-height: calc(100vh - 40px);
		}

		.modal.window-mode {
			height: 100vh;
			max-height: none;
			padding: 0 18px 18px;
		}

		.modal.window-mode .settings-body {
			display: flex;
		}
	}
</style>
