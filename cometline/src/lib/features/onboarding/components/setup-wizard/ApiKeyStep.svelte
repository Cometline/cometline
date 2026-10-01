<script lang="ts">
	import { Check, LoaderCircle, LogIn, RefreshCw } from '@lucide/svelte';
	import SettingsButton from '$lib/features/settings/components/SettingsButton.svelte';
	import type { ProviderConfig } from '$lib/types';
	import { openOllamaDownloadPage } from '$lib/ollama/client';
	import { providerLabel } from '$lib/features/onboarding/setup-wizard';
	import type { SetupWizardAuth } from '$lib/features/onboarding/setup-wizard-auth.svelte';
	import type { SetupWizardMemory } from '$lib/features/onboarding/setup-wizard-memory.svelte';
	import ProviderTabs from './ProviderTabs.svelte';
	import StepIntro from './StepIntro.svelte';
	import WizardField from './WizardField.svelte';

	let {
		providers,
		activeProviderId,
		provider,
		auth,
		memory,
		showApiKey,
		isConnected,
		onShowProvider,
		onPatchProvider,
		onToggleShowApiKey
	}: {
		providers: ProviderConfig[];
		activeProviderId: string;
		provider: ProviderConfig | undefined;
		auth: SetupWizardAuth;
		memory: SetupWizardMemory;
		showApiKey: boolean;
		isConnected: (provider: ProviderConfig) => boolean;
		onShowProvider: (id: string) => void;
		onPatchProvider: (id: string, patch: Partial<ProviderConfig>) => void;
		onToggleShowApiKey: () => void;
	} = $props();
</script>

{#snippet connectedBadge(tab: ProviderConfig)}
	{#if isConnected(tab)}<Check size={12} />{/if}
{/snippet}

<ProviderTabs {providers} {activeProviderId} onSelect={onShowProvider} badge={connectedBadge} />
{#if provider}
	{#if provider.method === 'codex'}
		<StepIntro>
			Sign in with your ChatGPT Plus/Pro browser session. Cometline stores a local
			Codex-compatible session at <code>~/.codex/auth.json</code>. No API key or Codex CLI
			install is required.
		</StepIntro>
		{#if auth.codexAuthStatus}
			<p class="codex-status" class:ok={auth.codexAuthStatus.authenticated}>
				{auth.codexAuthStatus.authenticated
					? 'Signed in with ChatGPT browser session.'
					: (auth.codexAuthStatus.error ?? 'Not signed in.')}
			</p>
		{/if}
		<div class="inline-actions">
			<SettingsButton
				variant="primary"
				onclick={auth.startCodexLogin}
				disabled={auth.startingCodexLogin || !window.electronAPI?.startCodexLogin}
			>
				{#if auth.startingCodexLogin}<LoaderCircle size={14} class="spin" />{:else}<LogIn
						size={14}
					/>{/if}
				Sign in with ChatGPT
			</SettingsButton>
			<SettingsButton
				variant="secondary"
				onclick={auth.refreshCodexAuthStatus}
				disabled={auth.checkingCodexAuth || !window.electronAPI?.getCodexAuthStatus}
			>
				{#if auth.checkingCodexAuth}<LoaderCircle size={14} class="spin" />{:else}<RefreshCw
						size={14}
					/>{/if}
				Check session
			</SettingsButton>
		</div>
	{:else if provider.method === 'xai'}
		<StepIntro>
			Sign in with your SuperGrok or X Premium subscription. Cometline stores the local xAI
			OAuth session at <code>~/.cometmind/xai/auth.json</code>.
		</StepIntro>
		{#if auth.xaiAuthStatus}
			<p class="codex-status" class:ok={auth.xaiAuthStatus.authenticated}>
				{auth.xaiAuthStatus.authenticated
					? 'Signed in with Grok subscription.'
					: (auth.xaiAuthStatus.error ?? 'Not signed in.')}
			</p>
		{/if}
		<div class="inline-actions">
			<SettingsButton
				variant="primary"
				onclick={auth.startXaiLogin}
				disabled={auth.startingXaiLogin || !window.electronAPI?.startXaiLogin}
			>
				{#if auth.startingXaiLogin}<LoaderCircle size={14} class="spin" />{:else}<LogIn
						size={14}
					/>{/if}
				Sign in with Grok
			</SettingsButton>
			<SettingsButton
				variant="secondary"
				onclick={auth.refreshXaiAuthStatus}
				disabled={auth.checkingXaiAuth || !window.electronAPI?.getXaiAuthStatus}
			>
				{#if auth.checkingXaiAuth}<LoaderCircle size={14} class="spin" />{:else}<RefreshCw
						size={14}
					/>{/if}
				Check session
			</SettingsButton>
		</div>
	{:else if provider.method === 'ollama'}
		<StepIntro>
			Ollama runs models on your Mac — no API key. Install the official app if needed, launch
			it once, then check that the local daemon is ready.
		</StepIntro>
		{#if memory.ollamaHealth}
			<p class="codex-status" class:ok={memory.ollamaHealth.ok} aria-live="polite">
				{memory.ollamaHealth.ok
					? `Ollama is ready${memory.ollamaHealth.version ? ` (v${memory.ollamaHealth.version})` : ''}.`
					: 'Ollama is not installed or not running.'}
			</p>
		{/if}
		{#if memory.ollamaWizardError}
			<p class="wizard-error">{memory.ollamaWizardError}</p>
		{/if}
		<div class="inline-actions">
			<SettingsButton variant="secondary" onclick={() => void openOllamaDownloadPage()}>
				Install Ollama
			</SettingsButton>
			<SettingsButton
				variant="primary"
				onclick={() => void memory.refreshOllamaHealth()}
				disabled={memory.checkingOllama}
			>
				{#if memory.checkingOllama}<LoaderCircle size={14} class="spin" />{:else}<RefreshCw
						size={14}
					/>{/if}
				{memory.checkingOllama ? 'Checking…' : 'Check Ollama'}
			</SettingsButton>
		</div>
	{:else}
		<StepIntro>
			Enter your API key for {providerLabel(provider)}. It's stored locally and never sent
			anywhere except the provider.
		</StepIntro>
		<WizardField label="API key">
			<div class="api-key-row">
				<input
					type={showApiKey ? 'text' : 'password'}
					class="field-input"
					placeholder="Paste your API key"
					value={provider.apiKey}
					oninput={(e) => onPatchProvider(provider.id, { apiKey: e.currentTarget.value })}
				/>
				<button class="toggle-visibility" onclick={onToggleShowApiKey} type="button">
					{showApiKey ? 'Hide' : 'Show'}
				</button>
			</div>
		</WizardField>
		<WizardField label="Base URL">
			<input
				type="text"
				class="field-input"
				value={provider.baseURL}
				oninput={(e) => onPatchProvider(provider.id, { baseURL: e.currentTarget.value })}
			/>
		</WizardField>
	{/if}
{/if}

<style>
	.field-input {
		width: 100%;
		padding: 9px 11px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, var(--panel-bg));
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
	}

	.field-input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.api-key-row {
		display: flex;
		gap: 8px;
	}

	.api-key-row .field-input {
		flex: 1;
	}

	.toggle-visibility {
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, var(--panel-bg));
		color: var(--text-muted);
		font: inherit;
		font-size: 12px;
		padding: 0 12px;
		cursor: pointer;
	}

	.toggle-visibility:hover {
		color: var(--text-main);
	}

	.wizard-error {
		margin: 12px 0 0;
		font-size: 12px;
		color: var(--status-error);
	}

	.codex-status {
		margin: 0 0 14px;
		font-size: 13px;
		color: var(--status-error);
	}

	.codex-status.ok {
		color: var(--status-success);
	}

	.inline-actions {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}
</style>
