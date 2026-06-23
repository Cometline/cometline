<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import { untrack } from 'svelte';
	import { Check, ChevronRight, ChevronLeft, LoaderCircle, LogIn, RefreshCw, Sparkles, X } from '@lucide/svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { connectionState } from '$lib/stores/runtime.svelte';
	import { cloneProvider } from '$lib/settings/schema';
	import SettingsButton from '$lib/components/settings/SettingsButton.svelte';
	import {
		buildEmbeddingDropdownOptions,
		embeddingKeyForFields,
		embeddingOptionKey,
		embeddingProviderForMethod,
		mergeEmbeddingFields,
		savedEmbeddingFromApi,
		type SavedEmbeddingRef
	} from '$lib/embedding-models';
	import {
		defaultMemorySettings,
		getMemorySettings,
		type MemorySettings
	} from '$lib/client/cometmind';
	import type { ProviderConfig, ProviderMethod, ProviderSettings } from '$lib/types';

	const METHOD_LABELS: Record<ProviderMethod, string> = {
		openai: 'OpenAI',
		anthropic: 'Anthropic',
		'opencode-go': 'OpenCode Go',
		codex: 'ChatGPT Codex',
		'openai-compatible': 'OpenAI Compatible'
	};

	function methodNeedsApiKey(method: ProviderMethod) {
		return method !== 'codex';
	}

	function canFetchModels(provider: ProviderConfig) {
		if (settingsStore.isFetchingModels || !provider.baseURL.trim()) return false;
		return (
			provider.method === 'codex' ||
			provider.method === 'opencode-go' ||
			provider.apiKey.trim().length > 0
		);
	}

	type Step = 'provider' | 'apikey' | 'model' | 'embedding' | 'connect';
	const STEP_ORDER: Step[] = ['provider', 'apikey', 'model', 'embedding', 'connect'];
	const STEP_TITLES: Record<Step, string> = {
		provider: 'Choose a provider',
		apikey: 'Connect your account',
		model: 'Pick a model',
		embedding: 'Memory embeddings (optional)',
		connect: 'Ready to go'
	};

	let step = $state<Step>('provider');
	let saving = $state(false);
	let saveError = $state('');
	let connecting = $state(false);
	let manualModel = $state('');
	let showApiKey = $state(false);

	// Codex browser-session auth state (mirrors SettingsProvidersPanel).
	type CodexAuthStatus = {
		authenticated: boolean;
		authPath: string;
		accountID?: string;
		error?: string;
	};
	let codexAuthStatus = $state<CodexAuthStatus | undefined>();
	let checkingCodexAuth = $state(false);
	let startingCodexLogin = $state(false);

	// Memory embedding state.
	let memorySettings = $state<MemorySettings | null>(null);
	let memoryLoading = $state(false);
	let memoryError = $state('');
	let selectedEmbeddingKey = $state('');
	let savedEmbedding = $state<SavedEmbeddingRef | undefined>();

	// Draft built from current settings so the wizard edits don't leak if
	// the user cancels.
	let draft = $state<ProviderSettings>(
		JSON.parse(JSON.stringify(settingsStore.settings)) as ProviderSettings
	);

	let selectedProviderId = $state(
		untrack(() => draft.providers.find((p) => p.id === 'anthropic')?.id ?? draft.providers[0]?.id ?? '')
	);

	let selectedProvider = $derived(
		draft.providers.find((p) => p.id === selectedProviderId) ?? draft.providers[0]
	);

	let stepIndex = $derived(STEP_ORDER.indexOf(step));
	let canAdvance = $derived.by(() => {
		if (step === 'provider') return Boolean(selectedProvider);
		if (step === 'apikey') {
			if (!selectedProvider) return false;
			// Codex needs a signed-in browser session, not an API key.
			if (selectedProvider.method === 'codex') {
				return Boolean(codexAuthStatus?.authenticated);
			}
			return selectedProvider.apiKey.trim().length > 0;
		}
		if (step === 'model') return Boolean(selectedProvider) && selectedProvider.enabledModels.length > 0;
		// Embedding step is always skippable.
		if (step === 'embedding') return true;
		return true;
	});

	// Embedding dropdown options derived from enabled providers.
	let embeddingOptions = $derived(
		buildEmbeddingDropdownOptions(draft.providers, savedEmbedding, memorySettings?.embedding)
	);

	function patchSelected(patch: Partial<ProviderConfig>) {
		if (!selectedProvider) return;
		draft = {
			...draft,
			providers: draft.providers.map((p) =>
				p.id === selectedProvider.id ? cloneProvider({ ...p, ...patch }) : p
			)
		};
	}

	function selectProvider(id: string) {
		selectedProviderId = id;
		// Reset model state when switching providers.
		manualModel = '';
		patchSelected({ enabledModels: [], selectedModel: '' });
	}

	function next() {
		const idx = STEP_ORDER.indexOf(step);
		if (idx < STEP_ORDER.length - 1) {
			const nextStep = STEP_ORDER[idx + 1];
			step = nextStep;
			// Auto-check Codex auth status when entering the apikey step for codex.
			if (nextStep === 'apikey' && selectedProvider?.method === 'codex') {
				void refreshCodexAuthStatus();
			}
		}
	}

	function back() {
		const idx = STEP_ORDER.indexOf(step);
		if (idx > 0) {
			step = STEP_ORDER[idx - 1];
		}
	}

	async function fetchModels() {
		if (!selectedProvider || !canFetchModels(selectedProvider)) return;
		try {
			const updated = await settingsStore.fetchModelsFor(selectedProvider);
			patchSelected({
				models: updated.models,
				enabledModels: updated.enabledModels,
				selectedModel: updated.selectedModel
			});
		} catch {
			// Error surfaced via settingsStore.error; keep the wizard usable.
		}
	}

	function selectModel(model: string) {
		if (!selectedProvider) return;
		patchSelected({ enabledModels: [model], selectedModel: model });
	}

	function addManualModel() {
		const model = manualModel.trim();
		if (!model || !selectedProvider) return;
		const models = selectedProvider.models.includes(model)
			? selectedProvider.models
			: [...selectedProvider.models, model];
		patchSelected({ models, enabledModels: [model], selectedModel: model });
		manualModel = '';
	}

	// --- Codex browser-session auth ---

	async function refreshCodexAuthStatus() {
		if (!window.electronAPI?.getCodexAuthStatus || checkingCodexAuth) return;
		checkingCodexAuth = true;
		try {
			codexAuthStatus = await window.electronAPI.getCodexAuthStatus();
		} finally {
			checkingCodexAuth = false;
		}
	}

	async function startCodexLogin() {
		if (!window.electronAPI?.startCodexLogin || startingCodexLogin) return;
		startingCodexLogin = true;
		try {
			await window.electronAPI.startCodexLogin();
			// Give the browser a moment, then refresh status.
			setTimeout(() => void refreshCodexAuthStatus(), 1500);
		} catch (err) {
			codexAuthStatus = {
				authenticated: false,
				authPath: '',
				error: err instanceof Error ? err.message : 'Failed to start Codex login.'
			};
		} finally {
			startingCodexLogin = false;
		}
	}

	// --- Memory embedding ---

	async function loadMemorySettings() {
		if (memoryLoading || memorySettings) return;
		memoryLoading = true;
		memoryError = '';
		try {
			const s = await getMemorySettings();
			memorySettings = s;
			savedEmbedding = savedEmbeddingFromApi(s.embedding);
			selectedEmbeddingKey = embeddingKeyForFields(
				draft.providers,
				mergeEmbeddingFields(s.embedding, savedEmbedding),
				savedEmbedding
			);
		} catch (err) {
			memoryError = err instanceof Error ? err.message : 'Failed to load memory settings';
			memorySettings = defaultMemorySettings();
		} finally {
			memoryLoading = false;
		}
	}

	function selectEmbedding(key: string) {
		selectedEmbeddingKey = key;
	}

	function applyEmbeddingSelection(): MemorySettings | null {
		if (!memorySettings) return null;
		if (!selectedEmbeddingKey) {
			// No selection — clear embedding fields.
			return {
				...memorySettings,
				embedding: { provider_id: '', provider: '', model: '', base_url: '', api_key: '' }
			};
		}
		const option = embeddingOptions.find((opt) => embeddingOptionKey(opt) === selectedEmbeddingKey);
		if (!option) return memorySettings;
		return {
			...memorySettings,
			embedding: {
				provider_id: option.providerId,
				provider: embeddingProviderForMethod(option.method),
				model: option.model,
				base_url: option.baseURL,
				api_key: option.apiKey
			}
		};
	}

	async function saveAndConnect() {
		if (!selectedProvider) return;
		// Enable the chosen provider and set it as active + default.
		const finalProviders = draft.providers.map((p) =>
			p.id === selectedProvider.id
				? cloneProvider({ ...p, enabled: true })
				: p
		);
		const finalDraft: ProviderSettings = {
			...draft,
			providers: finalProviders,
			activeProviderId: selectedProvider.id,
			defaultProviderId: selectedProvider.id,
			defaultModelId: selectedProvider.enabledModels[0] ?? selectedProvider.selectedModel
		};

		// Apply embedding selection (may be empty = skipped).
		const memoryPayload = applyEmbeddingSelection();
		const hasEmbedding = Boolean(memoryPayload?.embedding.model.trim());

		saving = true;
		saveError = '';
		try {
			await settingsStore.save(finalDraft, {
				restartCometMind: true,
				memory: hasEmbedding ? memoryPayload ?? undefined : undefined
			});
			draft = JSON.parse(JSON.stringify(settingsStore.settings)) as ProviderSettings;
			// Poll until the sidecar is healthy after restart.
			connecting = true;
			connectionState.reconnect();
			const ready = await waitForReady(20);
			connecting = false;
			if (ready) {
				await settingsStore.markSetupComplete();
				shellStore.closeSetup();
			} else {
				saveError = 'CometMind is still starting up. Your settings were saved — try sending a message in a moment.';
			}
		} catch (err) {
			saveError = err instanceof Error ? err.message : 'Failed to save settings.';
		} finally {
			saving = false;
			connecting = false;
		}
	}

	async function waitForReady(maxAttempts: number): Promise<boolean> {
		for (let i = 0; i < maxAttempts; i++) {
			if (connectionState.status === 'ready') return true;
			await new Promise((resolve) => setTimeout(resolve, 500));
		}
		return false;
	}

	function skip() {
		shellStore.closeSetup();
	}

	// Load memory settings when the embedding step is entered.
	$effect(() => {
		if (step === 'embedding') {
			void loadMemorySettings();
		}
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			skip();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="wizard-layer" transition:fade={{ duration: 120 }}>
	<button class="scrim" aria-label="Close setup wizard" onclick={skip}></button>
	<div
		class="wizard-modal"
		role="dialog"
		aria-modal="true"
		aria-labelledby="wizard-title"
		transition:scale={{ start: 0.97, duration: 140 }}
	>
		<header>
			<div class="title-mark"><Sparkles size={16} /></div>
			<div>
				<h2 id="wizard-title">Welcome to Cometline</h2>
				<p>{STEP_TITLES[step]} — step {stepIndex + 1} of {STEP_ORDER.length}</p>
			</div>
			<button class="close-btn" aria-label="Close" onclick={skip}><X size={16} /></button>
		</header>

		<div class="step-body scrollbar-none">
			{#if step === 'provider'}
				<p class="step-intro">
					Pick the LLM provider you'd like to use. You can add more later in Settings.
				</p>
				<div class="provider-list">
					{#each draft.providers as provider (provider.id)}
						<button
							class="provider-option"
							class:selected={provider.id === selectedProviderId}
							onclick={() => selectProvider(provider.id)}
						>
							<div class="provider-option-copy">
								<strong>{provider.name || METHOD_LABELS[provider.method]}</strong>
								<span>{provider.baseURL || 'Custom endpoint'}</span>
							</div>
							{#if provider.id === selectedProviderId}
								<Check size={16} />
							{/if}
						</button>
					{/each}
				</div>
		{:else if step === 'apikey'}
			{#if selectedProvider}
				{#if selectedProvider.method === 'codex'}
					<p class="step-intro">
						Sign in with your ChatGPT Plus/Pro browser session. Cometline stores a
						local Codex-compatible session at <code>~/.codex/auth.json</code>. No API key
						or Codex CLI install is required.
					</p>
					{#if codexAuthStatus}
						<p class="codex-status" class:ok={codexAuthStatus.authenticated}>
							{codexAuthStatus.authenticated
								? 'Signed in with ChatGPT browser session.'
								: (codexAuthStatus.error ?? 'Not signed in.')}
						</p>
					{/if}
					<div class="inline-actions">
						<SettingsButton
							variant="primary"
							onclick={startCodexLogin}
							disabled={startingCodexLogin || !window.electronAPI?.startCodexLogin}
						>
							{#if startingCodexLogin}<LoaderCircle size={14} class="spin" />{:else}<LogIn size={14} />{/if}
							Sign in with ChatGPT
						</SettingsButton>
						<SettingsButton
							variant="secondary"
							onclick={refreshCodexAuthStatus}
							disabled={checkingCodexAuth || !window.electronAPI?.getCodexAuthStatus}
						>
							{#if checkingCodexAuth}<LoaderCircle size={14} class="spin" />{:else}<RefreshCw size={14} />{/if}
							Check session
						</SettingsButton>
					</div>
				{:else}
					<p class="step-intro">
						Enter your API key for {selectedProvider.name || METHOD_LABELS[selectedProvider.method]}.
						It's stored locally and never sent anywhere except the provider.
					</p>
					<label class="field">
						<span class="field-label">API key</span>
						<div class="api-key-row">
							<input
								type={showApiKey ? 'text' : 'password'}
								class="field-input"
								placeholder="Paste your API key"
								value={selectedProvider.apiKey}
								oninput={(e) => patchSelected({ apiKey: e.currentTarget.value })}
							/>
							<button
								class="toggle-visibility"
								onclick={() => (showApiKey = !showApiKey)}
								type="button"
							>
								{showApiKey ? 'Hide' : 'Show'}
							</button>
						</div>
					</label>
					<label class="field">
						<span class="field-label">Base URL</span>
						<input
							type="text"
							class="field-input"
							value={selectedProvider.baseURL}
							oninput={(e) => patchSelected({ baseURL: e.currentTarget.value })}
						/>
					</label>
				{/if}
			{/if}
			{:else if step === 'model'}
				{#if selectedProvider}
					<p class="step-intro">
						Choose which model to use. Fetch the available list from the provider, or type a model ID manually.
					</p>
					<div class="fetch-row">
						<button
							class="fetch-btn"
							onclick={fetchModels}
							disabled={!canFetchModels(selectedProvider)}
						>
							{#if settingsStore.isFetchingModels}<LoaderCircle size={14} class="spin" />{/if}
							Fetch models
						</button>
						{#if selectedProvider.models.length > 0}
							<span class="fetch-hint">{selectedProvider.models.length} models available</span>
						{/if}
					</div>
					{#if selectedProvider.models.length > 0}
						<div class="model-list scrollbar-none">
							{#each selectedProvider.models as model (model)}
								<button
									class="model-option"
									class:selected={selectedProvider.enabledModels.includes(model)}
									onclick={() => selectModel(model)}
								>
									<span>{model}</span>
									{#if selectedProvider.enabledModels.includes(model)}
										<Check size={14} />
									{/if}
								</button>
							{/each}
						</div>
					{/if}
					<div class="manual-model">
						<input
							type="text"
							class="field-input"
							placeholder="Or type a model ID (e.g. claude-sonnet-4-5)"
							value={manualModel}
							oninput={(e) => (manualModel = e.currentTarget.value)}
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									e.preventDefault();
									addManualModel();
								}
							}}
						/>
						<button class="manual-add" onclick={addManualModel} disabled={!manualModel.trim()}>
							Add
						</button>
					</div>
				{/if}
			{:else if step === 'embedding'}
				<p class="step-intro">
					Cometline can store and retrieve memories across sessions using an embedding model.
					Pick one from your enabled providers, or skip this step — you can configure it later
					in Settings → Memory.
				</p>
				{#if memoryLoading}
					<p class="embedding-loading"><LoaderCircle size={14} class="spin" /> Loading memory settings…</p>
				{:else if memoryError}
					<p class="wizard-error">{memoryError}</p>
					<p class="step-intro">You can skip this step and configure memory later in Settings.</p>
				{:else if embeddingOptions.length === 0}
					<p class="step-intro">
						No embedding models are available from your enabled providers. To use memory,
						enable a provider with an embedding model (e.g. OpenAI's
						<code>text-embedding-3-small</code>) in Settings later.
					</p>
				{:else}
					<label class="field">
						<span class="field-label">Embedding model</span>
						<select class="field-input" value={selectedEmbeddingKey} onchange={(e) => selectEmbedding(e.currentTarget.value)}>
							<option value="">— Skip (configure later) —</option>
							{#each embeddingOptions as opt (embeddingOptionKey(opt))}
								<option value={embeddingOptionKey(opt)}>{opt.providerName} · {opt.model}</option>
							{/each}
						</select>
					</label>
				{/if}
			{:else if step === 'connect'}
				<p class="step-intro">
					{selectedProvider?.name || 'Your provider'} is ready to go. Save your settings and
					Cometline will connect.
				</p>
				<div class="review">
					<div class="review-row">
						<span>Provider</span>
						<strong>{selectedProvider?.name || METHOD_LABELS[selectedProvider?.method ?? 'anthropic']}</strong>
					</div>
					<div class="review-row">
						<span>Model</span>
						<strong>{(selectedProvider?.enabledModels[0] ?? selectedProvider?.selectedModel) || '—'}</strong>
					</div>
					<div class="review-row">
						<span>API key</span>
						<strong>{selectedProvider?.method === 'codex' ? (codexAuthStatus?.authenticated ? 'ChatGPT session' : 'Not signed in') : (selectedProvider?.apiKey ? 'Set' : 'Missing')}</strong>
					</div>
					<div class="review-row">
						<span>Embedding</span>
						<strong>{selectedEmbeddingKey ? embeddingOptions.find((o) => embeddingOptionKey(o) === selectedEmbeddingKey)?.model ?? '—' : 'Skipped'}</strong>
					</div>
				</div>
				{#if saveError}
					<p class="wizard-error">{saveError}</p>
				{/if}
			{/if}
		</div>

		<footer>
			<SettingsButton variant="secondary" onclick={skip}>Skip setup</SettingsButton>
			<div class="footer-right">
				{#if stepIndex > 0}
					<SettingsButton variant="secondary" onclick={back}>
						<ChevronLeft size={14} />
						Back
					</SettingsButton>
				{/if}
				{#if step !== 'connect'}
					<SettingsButton variant="primary" onclick={next} disabled={!canAdvance}>
						Next
						<ChevronRight size={14} />
					</SettingsButton>
				{:else}
					<SettingsButton variant="primary" onclick={saveAndConnect} disabled={saving || connecting}>
						{#if saving || connecting}<LoaderCircle size={14} class="spin" />{/if}
						{connecting ? 'Connecting…' : saving ? 'Saving…' : 'Save & connect'}
					</SettingsButton>
				{/if}
			</div>
		</footer>
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
		background: var(--panel-bg, #fff);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card, 16px);
		box-shadow: var(--shadow-card);
		overflow: hidden;
	}

	header {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 18px 20px;
		border-bottom: 1px solid var(--border-soft);
	}

	.title-mark {
		width: 32px;
		height: 32px;
		border-radius: 11px;
		background: rgba(0, 102, 204, 0.09);
		color: var(--accent);
		display: grid;
		place-items: center;
		flex-shrink: 0;
	}

	header h2 {
		margin: 0;
		font-size: 17px;
		font-weight: 600;
		color: var(--text-main);
	}

	header p {
		margin: 2px 0 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.close-btn {
		margin-left: auto;
		border: none;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
		padding: 4px;
		border-radius: 6px;
		display: grid;
		place-items: center;
	}

	.close-btn:hover {
		background: rgba(15, 23, 42, 0.05);
		color: var(--text-main);
	}

	.step-body {
		padding: 20px;
		overflow-y: auto;
		flex: 1;
	}

	.step-intro {
		margin: 0 0 16px;
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.provider-list {
		display: grid;
		gap: 8px;
	}

	.provider-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 12px 14px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg, #fff);
		cursor: pointer;
		text-align: left;
		transition: border-color 0.15s ease;
	}

	.provider-option:hover {
		border-color: rgba(0, 102, 204, 0.3);
	}

	.provider-option.selected {
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.provider-option-copy {
		display: grid;
		gap: 2px;
	}

	.provider-option-copy strong {
		font-size: 13px;
		font-weight: 600;
		color: var(--text-main);
	}

	.provider-option-copy span {
		font-size: 11px;
		color: var(--text-muted);
	}

	.field {
		display: grid;
		gap: 6px;
		margin-bottom: 16px;
	}

	.field-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	.field-input {
		width: 100%;
		padding: 9px 11px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, #fff);
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
		background: var(--panel-bg, #fff);
		color: var(--text-muted);
		font: inherit;
		font-size: 12px;
		padding: 0 12px;
		cursor: pointer;
	}

	.toggle-visibility:hover {
		color: var(--text-main);
	}

	.fetch-row {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 14px;
	}

	.fetch-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 14px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, #fff);
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}

	.fetch-btn:hover:not(:disabled) {
		border-color: var(--accent);
	}

	.fetch-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.fetch-hint {
		font-size: 12px;
		color: var(--text-muted);
	}

	.model-list {
		display: grid;
		gap: 6px;
		margin-bottom: 16px;
		max-height: 220px;
		overflow-y: auto;
	}

	.model-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 9px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, #fff);
		cursor: pointer;
		font: inherit;
		font-size: 12px;
		color: var(--text-main);
		text-align: left;
	}

	.model-option:hover {
		border-color: rgba(0, 102, 204, 0.3);
	}

	.model-option.selected {
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.manual-model {
		display: flex;
		gap: 8px;
	}

	.manual-model .field-input {
		flex: 1;
	}

	.manual-add {
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, #fff);
		color: var(--text-main);
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		padding: 0 14px;
		cursor: pointer;
	}

	.manual-add:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.review {
		display: grid;
		gap: 10px;
		padding: 16px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
	}

	.review-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 13px;
	}

	.review-row span {
		color: var(--text-muted);
	}

	.review-row strong {
		color: var(--text-main);
		font-weight: 600;
	}

	.wizard-error {
		margin: 12px 0 0;
		font-size: 12px;
		color: #b42318;
	}

	.codex-status {
		margin: 0 0 14px;
		font-size: 13px;
		color: #b42318;
	}

	.codex-status.ok {
		color: #067647;
	}

	.inline-actions {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}

	.embedding-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text-muted);
	}

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

	:global(.spin) {
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
