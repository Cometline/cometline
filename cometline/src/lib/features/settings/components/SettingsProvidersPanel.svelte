<script lang="ts">
	import { Plus, Trash2 } from '@lucide/svelte';
	import type { ProviderConfig, ProviderMethod } from '$lib/types';
	import ProviderCard from './ProviderCard.svelte';
	import ProviderConnectionFields from './providers/ProviderConnectionFields.svelte';
	import ProviderModelSection from './providers/ProviderModelSection.svelte';

	const METHOD_LABELS: Record<ProviderMethod, string> = {
		openai: 'OpenAI',
		anthropic: 'Anthropic',
		'opencode-go': 'OpenCode Go',
		codex: 'ChatGPT Codex',
		xai: 'xAI Grok Subscription',
		ollama: 'Ollama Local',
		'openai-compatible': 'Advanced / Custom endpoint'
	};

	const DEFAULT_PROVIDER_IDS = new Set([
		'anthropic',
		'openai',
		'opencode-go',
		'codex',
		'xai',
		'ollama',
		'openai-compatible'
	]);

	type CodexAuthStatus = {
		authenticated: boolean;
		authPath: string;
		accountID?: string;
		error?: string;
	};
	type XaiAuthStatus = {
		authenticated: boolean;
		authPath: string;
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
		xaiAuthStatus,
		checkingXaiAuth = false,
		startingXaiLogin = false,
		onAddProvider,
		onRemoveProvider,
		onToggleProvider,
		onUpdateSelected,
		onSetMethod,
		onFetchModels,
		onToggleModel,
		onStartCodexLogin,
		onRefreshCodexAuth,
		onStartXaiLogin,
		onRefreshXaiAuth
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
		xaiAuthStatus?: XaiAuthStatus;
		checkingXaiAuth?: boolean;
		startingXaiLogin?: boolean;
		onAddProvider: () => void;
		onRemoveProvider: (id: string) => void;
		onToggleProvider: (id: string) => void;
		onUpdateSelected: (patch: Partial<ProviderConfig>) => void;
		onSetMethod: (method: ProviderMethod) => void;
		onFetchModels: () => void;
		onToggleModel: (model: string) => void;
		onStartCodexLogin: () => void;
		onRefreshCodexAuth: () => void;
		onStartXaiLogin: () => void;
		onRefreshXaiAuth: () => void;
	} = $props();
</script>

<div class="provider-shell settings-panel-frame">
	<aside class="provider-sidebar">
		<div class="provider-sidebar-title">
			<span>{enabledProviderCount} enabled</span>
			<button class="icon-button inline" aria-label="Add provider" onclick={onAddProvider}>
				<Plus size={15} />
			</button>
		</div>

		<div class="provider-list scrollbar-none">
			{#each providers as provider (provider.id)}
				<ProviderCard
					name={provider.name}
					method={provider.method}
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
		<section class="provider-detail scrollbar-none">
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

			<ProviderConnectionFields
				provider={selectedProvider}
				{codexAuthStatus}
				{checkingCodexAuth}
				{startingCodexLogin}
				{xaiAuthStatus}
				{checkingXaiAuth}
				{startingXaiLogin}
				onUpdate={onUpdateSelected}
				{onSetMethod}
				{onStartCodexLogin}
				{onRefreshCodexAuth}
				{onStartXaiLogin}
				{onRefreshXaiAuth}
			/>

			<ProviderModelSection
				provider={selectedProvider}
				bind:modelSearch
				{filteredModels}
				xaiAuthenticated={Boolean(xaiAuthStatus?.authenticated)}
				onUpdate={onUpdateSelected}
				{onFetchModels}
				{onToggleModel}
			/>
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
		background: var(--color-7aa1aa);
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

	.empty-providers {
		padding: 12px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
