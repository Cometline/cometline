<script lang="ts">
	import { ChevronLeft, ChevronRight, LoaderCircle } from '@lucide/svelte';
	import SettingsButton from '$lib/features/settings/components/SettingsButton.svelte';
	import type { SetupWizardStep } from '$lib/features/onboarding/setup-wizard';

	let {
		step,
		stepIndex,
		canAdvance,
		settingsSaved,
		saving,
		connecting,
		onSkip,
		onBack,
		onNext,
		onContinue,
		onSaveAndConnect,
		onFinish
	}: {
		step: SetupWizardStep;
		stepIndex: number;
		canAdvance: boolean;
		settingsSaved: boolean;
		saving: boolean;
		connecting: boolean;
		onSkip: () => void;
		onBack: () => void;
		onNext: () => void;
		onContinue: () => void;
		onSaveAndConnect: () => Promise<void>;
		onFinish: () => void;
	} = $props();
</script>

<footer>
	<SettingsButton variant="secondary" onclick={onSkip}>Skip setup</SettingsButton>
	<div class="footer-right">
		{#if stepIndex > 0}
			<SettingsButton variant="secondary" onclick={onBack}>
				<ChevronLeft size={14} />
				Back
			</SettingsButton>
		{/if}
		{#if step === 'connect'}
			{#if settingsSaved}
				<SettingsButton variant="primary" onclick={onContinue}>
					Continue
					<ChevronRight size={14} />
				</SettingsButton>
			{:else}
				<SettingsButton
					variant="primary"
					onclick={onSaveAndConnect}
					disabled={saving || connecting}
				>
					{#if saving || connecting}<LoaderCircle size={14} class="spin" />{/if}
					{connecting ? 'Connecting…' : saving ? 'Saving…' : 'Save & connect'}
				</SettingsButton>
			{/if}
		{:else if step === 'permissions'}
			<SettingsButton variant="primary" onclick={onFinish}>Finish</SettingsButton>
		{:else}
			<SettingsButton variant="primary" onclick={onNext} disabled={!canAdvance}>
				Next
				<ChevronRight size={14} />
			</SettingsButton>
		{/if}
	</div>
</footer>

<style>
	footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 16px 20px;
		border-top: 1px solid var(--border-soft);
	}

	.footer-right {
		display: flex;
		gap: 8px;
	}
</style>
