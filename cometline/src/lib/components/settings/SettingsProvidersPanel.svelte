<script lang="ts">
	import { LogIn, LoaderCircle, Plus, RefreshCw, Trash2 } from '@lucide/svelte';
	import type { ProviderConfig, ProviderMethod } from '$lib/types';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import ProviderCard from './ProviderCard.svelte';
	import ModelRow from './ModelRow.svelte';

	const METHOD_LABELS: Record<ProviderMethod, string> = {
		openai: 'OpenAI',
		anthropic: 'Anthropic',
		'opencode-go': 'OpenCode Go',
		codex: 'ChatGPT Codex',
		'openai-compatible': 'OpenAI-compatible'
	};

	const DEFAULT_PROVIDER_IDS = new Set([
		'anthropic',
		'openai',
		'opencode-go',
		'codex',
		'openai-compatible'
	]);

	type CodexAuthStatus = {
		authenticated: boolean;
		authPath: string;
		accountID?: string;
		error?: string;
	};

	let {
		providers,
		selectedProviderId = $bindable(''),
		modelSearch = $bindable(''),
		enabledProviderCount,
		filteredModels,
		selectedProvider,
		codexAuthStatus,
		checkingCodexAuth = false,
		startingCodexLogin = false,
		onAddProvider,
		onRemoveProvider,
		onToggleProvider,
		onUpdateSelected,
		onSetMethod,
		onFetchModels,
		onToggleModel,
		onStartCodexLogin,
		onRefreshCodexAuth
	}: {
		providers: ProviderConfig[];
		selectedProviderId?: string;
		modelSearch?: string;
		enabledProviderCount: number;
		filteredModels: string[];
		selectedProvider: ProviderConfig | undefined;
		codexAuthStatus?: CodexAuthStatus;
		checkingCodexAuth?: boolean;
		startingCodexLogin?: boolean;
		onAddProvider: () => void;
		onRemoveProvider: (id: string) => void;
		onToggleProvider: (id: string) => void;
		onUpdateSelected: (patch: Partial<ProviderConfig>) => void;
		onSetMethod: (method: ProviderMethod) => void;
		onFetchModels: () => void;
		onToggleModel: (model: string) => void;
		onStartCodexLogin: () => void;
		onRefreshCodexAuth: () => void;
	} = $props();

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
</script>

<div class="provider-shell settings-panel-frame">
	<aside class="provider-sidebar">
		<div class="provider-sidebar-title">
			<span>{enabledProviderCount} enabled</span>
			<button class="icon-button inline" aria-label="Add provider" onclick={onAddProvider}>
				<Plus size={15} />
			</button>
		</div>

		<div class="provider-list">
			{#each providers as provider (provider.id)}
				<ProviderCard
					name={provider.name}
					selected={selectedProviderId === provider.id}
					enabled={provider.enabled}
					onclick={() => {
						selectedProviderId = provider.id;
						modelSearch = '';
					}}
				/>
			{:else}
				<p class="empty-providers">No providers configured.</p>
			{/each}
		</div>
	</aside>

	{#if selectedProvider}
		<section class="provider-detail">
			<div class="detail-heading">
				<div>
					<h3>{selectedProvider.name}</h3>
					<p>
						{METHOD_LABELS[selectedProvider.method]} · {selectedProvider.enabledModels
							.length}
						enabled models
					</p>
				</div>
				<div class="detail-actions">
					{#if !DEFAULT_PROVIDER_IDS.has(selectedProvider.id)}
						<button
							class="secondary danger"
							aria-label="Delete provider"
							onclick={() => onRemoveProvider(selectedProvider.id)}
						>
							<Trash2 size={14} />
						</button>
					{/if}
					<button
						class="switch"
						class:on={selectedProvider.enabled}
						role="switch"
						aria-checked={selectedProvider.enabled}
						aria-label={`${selectedProvider.enabled ? 'Disable' : 'Enable'} ${selectedProvider.name}`}
						title={`${selectedProvider.enabled ? 'Disable' : 'Enable'} ${selectedProvider.name}`}
						onclick={() => onToggleProvider(selectedProvider.id)}
					>
						<span></span>
					</button>
				</div>
			</div>

			<div class="form-grid">
				<label>
					<span>Name</span>
					<input
						value={selectedProvider.name}
						oninput={(e) => onUpdateSelected({ name: e.currentTarget.value })}
						placeholder="Provider name"
						spellcheck="false"
					/>
				</label>

				<label>
					<span>Method</span>
					<select
						value={selectedProvider.method}
						onchange={(e) => onSetMethod(e.currentTarget.value as ProviderMethod)}
					>
						<option value="codex">ChatGPT Codex</option>
						<option value="openai">OpenAI</option>
						<option value="anthropic">Anthropic</option>
						<option value="opencode-go">OpenCode Go</option>
						<option value="openai-compatible">OpenAI-compatible</option>
					</select>
				</label>

				<label>
					<span>Base URL</span>
					<input
						value={selectedProvider.baseURL}
						oninput={(e) => onUpdateSelected({ baseURL: e.currentTarget.value })}
						placeholder="https://example.com/v1"
						spellcheck="false"
					/>
				</label>

				{#if methodNeedsApiKey(selectedProvider.method)}
					<label>
						<span>API Key</span>
						<input
							value={selectedProvider.apiKey}
							oninput={(e) => onUpdateSelected({ apiKey: e.currentTarget.value })}
							type="password"
							placeholder="sk-..."
							spellcheck="false"
						/>
					</label>
				{:else}
					<div class="field-note">
						<span>Authentication</span>
						<p>
							Uses your ChatGPT Plus/Pro browser sign-in and stores a local
							Codex-compatible session at <code>~/.codex/auth.json</code>. No API key
							or Codex CLI install is required.
						</p>
						{#if codexAuthStatus}
							<p class:ok={codexAuthStatus.authenticated}>
								{codexAuthStatus.authenticated
									? 'Signed in with ChatGPT browser session.'
									: (codexAuthStatus.error ?? 'Not signed in.')}
							</p>
						{/if}
						<div class="inline-actions">
							<button
								class="secondary"
								type="button"
								onclick={onStartCodexLogin}
								disabled={startingCodexLogin ||
									!window.electronAPI?.startCodexLogin}
							>
								{#if startingCodexLogin}<span class="spin"
										><LoaderCircle size={14} /></span
									>{:else}<LogIn size={14} />{/if}
								Sign in with ChatGPT
							</button>
							<button
								class="secondary"
								type="button"
								onclick={onRefreshCodexAuth}
								disabled={checkingCodexAuth ||
									!window.electronAPI?.getCodexAuthStatus}
							>
								{#if checkingCodexAuth}<span class="spin"
										><LoaderCircle size={14} /></span
									>{:else}<RefreshCw size={14} />{/if}
								Check session
							</button>
						</div>
					</div>
				{/if}
			</div>

			<div class="settings-section model-section">
				<div class="settings-section-heading model-heading">
					<div>
						<h3>Models</h3>
						{#if selectedProvider.method === 'codex'}
							<p>
								Use Fetch models to refresh models from your ChatGPT browser
								session.
							</p>
						{:else if selectedProvider.method === 'opencode-go'}
							<p>
								Use Fetch models to refresh the latest list from <code>/models</code
								> at OpenCode Go.
							</p>
						{:else}
							<p>
								Use Fetch models to refresh the latest list from <code>/models</code
								>.
							</p>
						{/if}
					</div>
					<button
						class="secondary"
						onclick={onFetchModels}
						disabled={!canFetchModels(selectedProvider)}
					>
						{#if settingsStore.isFetchingModels}<span class="spin"
								><LoaderCircle size={14} /></span
							>{/if}
						Fetch models
					</button>
				</div>

				<input
					class="model-search"
					bind:value={modelSearch}
					placeholder="Search models..."
					spellcheck="false"
				/>

				<div class="settings-scroll-list model-list">
					{#each filteredModels as model (model)}
						<ModelRow
							{model}
							providerId={selectedProvider.id}
							enabled={selectedProvider.enabledModels.includes(model)}
							onclick={() => onToggleModel(model)}
						/>
					{:else}
						<p class="empty-models">
							{selectedProvider.models.length === 0
								? 'No models loaded yet.'
								: 'No models match your search.'}
						</p>
					{/each}
				</div>
			</div>
		</section>
	{/if}
</div>

<style>
	.provider-shell {
		display: grid;
		grid-template-columns: minmax(0, 220px) minmax(0, 1fr);
		gap: 14px;
		min-height: 0;
	}


	.provider-sidebar {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-height: 0;
		padding-right: 12px;
		border-right: 1px solid var(--border-soft);
	}

	.provider-sidebar-title {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.provider-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
		overflow: auto;
		min-height: 0;
	}

	.provider-detail {
		min-width: 0;
		overflow: auto;
	}

	.detail-heading {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 14px;
	}

	.detail-heading h3 {
		margin: 0;
		font-size: 16px;
	}

	.detail-heading p {
		margin: 4px 0 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.detail-actions {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
		margin-bottom: 16px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.field-note {
		display: grid;
		grid-column: 1 / -1;
		gap: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.55);
		padding: 10px 11px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.field-note span {
		font-weight: 700;
	}

	.field-note p {
		max-width: 640px;
		font-weight: 500;
		line-height: 1.45;
		margin: 0;
	}

	.field-note p.ok {
		color: #24745d;
		font-weight: 650;
	}

	.inline-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding-top: 2px;
	}

	input,
	select {
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.76);
		padding: 10px 11px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	.model-section {
		margin-top: 4px;
		padding-top: 20px;
		border-top: 1px solid var(--border-soft);
	}

	.model-heading {
		margin-bottom: 0;
	}

	.model-heading h3 {
		margin: 0;
		font-size: 14px;
	}

	.model-heading p {
		margin: 4px 0 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.model-heading code {
		font-size: 11px;
	}

	.model-search {
		width: 100%;
		margin-bottom: 10px;
	}

	.secondary {
		border: none;
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-main);
		cursor: pointer;
	}

	.secondary.danger:hover {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.switch {
		flex-shrink: 0;
		width: 44px;
		height: 28px;
		border: none;
		border-radius: 999px;
		background: rgba(203, 213, 225, 0.72);
		padding: 3px;
		display: flex;
		align-items: center;
		justify-content: flex-start;
		cursor: pointer;
	}

	.switch span {
		width: 22px;
		height: 22px;
		border-radius: 999px;
		background: white;
		box-shadow: 0 1px 5px rgba(15, 23, 42, 0.16);
	}

	.switch.on {
		justify-content: flex-end;
		background: #7aa1aa;
	}

	.icon-button {
		display: grid;
		place-items: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 8px;
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-muted);
		cursor: pointer;
	}

	.empty-models,
	.empty-providers {
		padding: 12px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
