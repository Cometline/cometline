<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import {
		embeddingReviewLabel,
		STEP_ORDER,
		STEP_TITLES
	} from '$lib/features/onboarding/setup-wizard';
	import { createSetupWizardController } from '$lib/features/onboarding/setup-wizard-controller.svelte';
	import WizardHeader from './setup-wizard/WizardHeader.svelte';
	import WizardFooter from './setup-wizard/WizardFooter.svelte';
	import ProviderStep from './setup-wizard/ProviderStep.svelte';
	import ApiKeyStep from './setup-wizard/ApiKeyStep.svelte';
	import ModelStep from './setup-wizard/ModelStep.svelte';
	import EmbeddingStep from './setup-wizard/EmbeddingStep.svelte';
	import ConnectStep from './setup-wizard/ConnectStep.svelte';
	import PermissionsStep from './setup-wizard/PermissionsStep.svelte';

	const wizard = createSetupWizardController();

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			wizard.skip();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="wizard-layer" transition:fade={{ duration: 120 }}>
	<button class="scrim" aria-label="Close setup wizard" onclick={wizard.skip}></button>
	<div
		class="wizard-modal"
		role="dialog"
		aria-modal="true"
		aria-labelledby="wizard-title"
		transition:scale={{ start: 0.97, duration: 140 }}
	>
		<WizardHeader
			subtitle="{STEP_TITLES[wizard.step]} — step {wizard.stepIndex +
				1} of {STEP_ORDER.length}"
			onClose={wizard.skip}
		/>

		<div class="step-body scrollbar-none">
			{#if wizard.step === 'provider'}
				<ProviderStep
					providers={wizard.draft.providers}
					selectedProviderIds={wizard.selectedProviderIds}
					defaultProviderId={wizard.defaultProviderId}
					onToggle={wizard.toggleProvider}
					onSetDefault={wizard.setDefaultProvider}
				/>
			{:else if wizard.step === 'apikey'}
				<ApiKeyStep
					providers={wizard.selectedProviders}
					activeProviderId={wizard.activeProviderId}
					provider={wizard.selectedProvider}
					auth={wizard.auth}
					memory={wizard.memory}
					showApiKey={wizard.showApiKey}
					isConnected={wizard.providerIsConnected}
					onShowProvider={wizard.showProvider}
					onPatchProvider={wizard.patchProvider}
					onToggleShowApiKey={wizard.toggleShowApiKey}
				/>
			{:else if wizard.step === 'model'}
				<ModelStep
					providers={wizard.selectedProviders}
					activeProviderId={wizard.activeProviderId}
					provider={wizard.selectedProvider}
					filteredModels={wizard.filteredModels}
					bind:modelFilter={wizard.modelFilter}
					canFetch={wizard.canFetchModels}
					fetching={settingsStore.isFetchingModels}
					onShowProvider={wizard.showProvider}
					onFetchModels={wizard.fetchModels}
					onSelectModel={wizard.selectModel}
				/>
			{:else if wizard.step === 'embedding'}
				<EmbeddingStep memory={wizard.memory} />
			{:else if wizard.step === 'connect'}
				<ConnectStep
					selectedProviders={wizard.selectedProviders}
					defaultProvider={wizard.defaultProvider}
					embeddingLabel={embeddingReviewLabel(
						wizard.memory.selectedEmbeddingKey,
						wizard.memory.availableEmbeddingOptions
					)}
					saveError={wizard.saveError}
				/>
			{:else if wizard.step === 'permissions'}
				<PermissionsStep
					saveError={wizard.saveError}
					screenCapturePreferred={wizard.screenCapturePreferred}
					screenCaptureStatus={wizard.screenCaptureStatus}
					screenCaptureBusy={wizard.screenCaptureBusy}
					onSetScreenCapture={wizard.setWizardScreenCapture}
				/>
			{/if}
		</div>

		<WizardFooter
			step={wizard.step}
			stepIndex={wizard.stepIndex}
			canAdvance={wizard.canAdvance}
			settingsSaved={wizard.settingsSaved}
			saving={wizard.saving}
			connecting={wizard.connecting}
			onSkip={wizard.skip}
			onBack={wizard.back}
			onNext={wizard.next}
			onContinue={wizard.continueToPermissions}
			onSaveAndConnect={wizard.saveAndConnect}
			onFinish={wizard.finishSetup}
		/>
	</div>
</div>

<style>
	.wizard-layer {
		position: fixed;
		inset: 0;
		z-index: 85;
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

	.wizard-modal {
		position: relative;
		width: min(560px, 100%);
		max-height: min(680px, 90vh);
		display: flex;
		flex-direction: column;
		background: var(--panel-bg, var(--panel-bg));
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card, 16px);
		box-shadow: var(--shadow-card);
		overflow: hidden;
	}

	.step-body {
		padding: 20px;
		overflow-y: auto;
		flex: 1;
	}

	:global(.spin) {
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
