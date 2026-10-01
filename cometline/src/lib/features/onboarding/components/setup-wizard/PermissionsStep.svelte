<script lang="ts">
	import SettingsButton from '$lib/features/settings/components/SettingsButton.svelte';
	import StepIntro from './StepIntro.svelte';

	let {
		saveError,
		screenCapturePreferred,
		screenCaptureStatus,
		screenCaptureBusy,
		onSetScreenCapture
	}: {
		saveError: string;
		screenCapturePreferred: boolean;
		screenCaptureStatus: string;
		screenCaptureBusy: boolean;
		onSetScreenCapture: (enabled: boolean) => Promise<void>;
	} = $props();
</script>

<StepIntro>
	Your provider settings are already saved. Cometline can capture your screen and show screenshots
	in chat when Screen & System Audio Recording is allowed. You can skip or finish now and enable
	this later in Settings → App — a restart after granting System Settings will not lose your
	provider setup.
</StepIntro>
{#if saveError}
	<p class="wizard-error">{saveError}</p>
{/if}
<label class="permission-toggle">
	<input
		type="checkbox"
		checked={screenCapturePreferred}
		disabled={screenCaptureBusy || !window.electronAPI?.setScreenCapturePreferred}
		onchange={(e) => void onSetScreenCapture(e.currentTarget.checked)}
	/>
	<span>
		<strong>Enable screen capture</strong>
		<small>
			Status: {screenCaptureStatus}
			{#if screenCaptureStatus === 'denied' || screenCaptureStatus === 'not-determined'}
				— you may need to approve Cometline in System Settings.
			{/if}
		</small>
	</span>
</label>
{#if window.electronAPI?.openScreenCaptureSettings}
	<div class="inline-actions permission-actions">
		<SettingsButton
			variant="secondary"
			onclick={() => void window.electronAPI?.openScreenCaptureSettings?.()}
		>
			Open System Settings…
		</SettingsButton>
	</div>
{/if}

<style>
	.permission-toggle {
		display: flex;
		gap: 12px;
		align-items: flex-start;
		padding: 12px 14px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		font-size: 13px;
	}

	.permission-toggle input {
		margin-top: 3px;
	}

	.permission-toggle span {
		display: grid;
		gap: 4px;
	}

	.permission-toggle small {
		color: var(--text-muted);
		font-size: 12px;
		line-height: 1.45;
	}

	.wizard-error {
		margin: 12px 0 0;
		font-size: 12px;
		color: var(--status-error);
	}

	.inline-actions {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}

	.permission-actions {
		margin-top: 12px;
	}
</style>
