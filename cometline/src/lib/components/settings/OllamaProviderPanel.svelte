<script lang="ts">
	import { onMount } from 'svelte';
	import { ExternalLink, LoaderCircle, RefreshCw } from '@lucide/svelte';
	import type { ProviderConfig } from '$lib/types';
	import {
		featuredOllamaCatalog,
		isValidOllamaModelName,
		OLLAMA_DEFAULT_NATIVE_BASE,
		type OllamaCatalogEntry
	} from '$lib/ollama/catalog';
	import {
		cancelOllamaPull,
		checkOllamaHealth,
		formatBytes,
		listOllamaModels,
		onOllamaPullProgress,
		openOllamaDownloadPage,
		pullOllamaModel,
		type OllamaHealthResult,
		type OllamaInstalledModel,
		type OllamaPullProgress
	} from '$lib/ollama/client';
	import SettingsButton from './SettingsButton.svelte';
	import ModelRow from './ModelRow.svelte';
	import { modelStore } from '$lib/stores/model.svelte';

	let {
		provider,
		onUpdate
	}: {
		provider: ProviderConfig;
		onUpdate: (patch: Partial<ProviderConfig>) => void;
	} = $props();

	let health = $state<OllamaHealthResult | null>(null);
	let checking = $state(false);
	let installed = $state<OllamaInstalledModel[]>([]);
	let error = $state('');
	let pullingId = $state('');
	let pullProgress = $state<OllamaPullProgress | null>(null);
	let customModel = $state('');
	let showAdvanced = $state(false);
	let showGuide = $state(false);

	const featured = featuredOllamaCatalog();
	const installedNames = $derived(new Set(installed.map((m) => m.name)));

	async function refresh() {
		checking = true;
		error = '';
		try {
			const next = await checkOllamaHealth(provider.baseURL || OLLAMA_DEFAULT_NATIVE_BASE);
			health = next;
			if (next.ok) {
				const listed = await listOllamaModels(next.baseURL);
				installed = listed.models;
				const names = listed.models.map((m) => m.name);
				const models = Array.from(new Set([...provider.models, ...names]));
				onUpdate({
					baseURL: next.baseURL,
					models,
					enabledModels: provider.enabledModels.filter((m) => models.includes(m))
				});
			} else {
				installed = [];
			}
		} catch (err) {
			error = err instanceof Error ? err.message : String(err);
			health = {
				ok: false,
				state: 'unreachable',
				baseURL: provider.baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
				error
			};
		} finally {
			checking = false;
		}
	}

	function isCancelledPull(err: unknown): boolean {
		return err instanceof Error && /cancelled/i.test(err.message);
	}

	async function pullEntry(entry: OllamaCatalogEntry) {
		pullingId = entry.id;
		error = '';
		pullProgress = { model: entry.pullName, status: 'starting' };
		try {
			const result = await pullOllamaModel({
				baseURL: provider.baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
				catalogId: entry.id
			});
			installed = result.models;
			const names = result.models.map((m) => m.name);
			onUpdate({
				models: Array.from(new Set([...provider.models, ...names])),
				enabled: true
			});
			await refresh();
		} catch (err) {
			if (!isCancelledPull(err)) {
				error = err instanceof Error ? err.message : String(err);
			}
		} finally {
			pullingId = '';
			pullProgress = null;
		}
	}

	async function pullCustom() {
		const name = customModel.trim();
		if (!isValidOllamaModelName(name)) {
			error = 'Enter a valid Ollama model name (e.g. llama3.2:3b)';
			return;
		}
		pullingId = `custom:${name}`;
		error = '';
		pullProgress = { model: name, status: 'starting' };
		try {
			const result = await pullOllamaModel({
				baseURL: provider.baseURL || OLLAMA_DEFAULT_NATIVE_BASE,
				modelName: name
			});
			installed = result.models;
			const names = result.models.map((m) => m.name);
			onUpdate({
				models: Array.from(new Set([...provider.models, ...names])),
				enabled: true
			});
			customModel = '';
			await refresh();
		} catch (err) {
			if (!isCancelledPull(err)) {
				error = err instanceof Error ? err.message : String(err);
			}
		} finally {
			pullingId = '';
			pullProgress = null;
		}
	}

	async function cancelPull() {
		await cancelOllamaPull();
	}

	function toggleModel(model: string) {
		const enabled = provider.enabledModels.includes(model)
			? provider.enabledModels.filter((m) => m !== model)
			: [...provider.enabledModels, model];
		onUpdate({
			enabledModels: enabled,
			selectedModel: enabled[0] || '',
			enabled: enabled.length > 0 ? true : provider.enabled
		});
	}

	onMount(() => {
		void refresh();
		return onOllamaPullProgress((payload) => {
			if (pullingId) pullProgress = payload;
		});
	});
</script>

<div class="ollama-panel">
	<div class="field-note" class:ok={Boolean(health?.ok)} class:bad={Boolean(health && !health.ok)}>
		<span>Local runtime</span>
		<p class="status-line">
			{#if !health}
				Checking Ollama…
			{:else if health.ok}
				Ollama is ready{health.version ? ` (v${health.version})` : ''}. Connected to
				<code>{health.baseURL}</code>.
			{:else}
				Ollama is not installed or not running. Install Ollama, launch it once, then click
				Check again.
			{/if}
		</p>
		<div class="inline-actions">
			<SettingsButton variant="secondary" disabled={checking} onclick={() => void refresh()}>
				{#if checking}
					<LoaderCircle size={14} class="spin" />
				{:else}
					<RefreshCw size={14} />
				{/if}
				Check again
			</SettingsButton>
			{#if !health?.ok}
				<SettingsButton variant="secondary" onclick={() => void openOllamaDownloadPage()}>
					<ExternalLink size={14} />
					Install Ollama
				</SettingsButton>
			{/if}
		</div>
	</div>

	{#if !health?.ok}
		<button class="link-button" type="button" onclick={() => (showGuide = !showGuide)}>
			{showGuide ? 'Hide' : 'Show'} setup guide
		</button>
		{#if showGuide}
			<ol class="guide settings-field-hint">
				<li>Open the official Ollama download page and install for macOS.</li>
				<li>Launch Ollama once so the local daemon listens on port 11434.</li>
				<li>Return here and click Check again — no API key is required.</li>
			</ol>
		{/if}
	{/if}

	{#if error}
		<p class="settings-field-hint error">{error}</p>
	{/if}

	{#if health?.ok}
		<section class="settings-section catalog-section">
			<div class="settings-section-heading">
				<div>
					<h4>Recommended models</h4>
					<p>Pull, then enable below and assign roles in Model Roles.</p>
				</div>
			</div>

			<div class="catalog-list">
				{#each featured as entry (entry.id)}
					{@const installedAlready = installedNames.has(entry.pullName)}
					{@const isPulling = pullingId === entry.id}
					<article class="catalog-item" class:pulling={isPulling}>
						<div class="catalog-item-body">
							<strong>{entry.displayName}</strong>
							<small>
								{entry.pullName}
								<span class="meta-sep">·</span>
								{entry.sizeLabel}
								{#if entry.architectureNote}
									<span class="meta-sep">·</span>
									{entry.architectureNote}
								{/if}
							</small>
							{#if isPulling && pullProgress}
								<div class="pull-inline">
									<div
										class="progress-track"
										role="progressbar"
										aria-valuemin={0}
										aria-valuemax={100}
										aria-valuenow={pullProgress.percent ?? 0}
									>
										<span
											style:width={`${pullProgress.percent != null ? pullProgress.percent : 8}%`}
											class:indeterminate={pullProgress.percent == null}
										></span>
									</div>
									<span class="pull-status">
										{#if pullProgress.percent != null}
											{pullProgress.percent}%{#if pullProgress.completed != null &&
												pullProgress.total}
												· {formatBytes(pullProgress.completed)} / {formatBytes(
													pullProgress.total
												)}{/if}
										{:else}
											{pullProgress.status || 'Starting…'}
										{/if}
									</span>
								</div>
							{/if}
						</div>
						<div class="catalog-item-action">
							{#if installedAlready}
								<span class="installed-label">Installed</span>
							{:else if isPulling}
								<SettingsButton
									variant="secondary"
									class="catalog-btn"
									onclick={() => void cancelPull()}
								>
									Cancel
								</SettingsButton>
							{:else}
								<SettingsButton
									variant="secondary"
									class="catalog-btn"
									disabled={Boolean(pullingId)}
									onclick={() => void pullEntry(entry)}
								>
									Pull
								</SettingsButton>
							{/if}
						</div>
					</article>
				{/each}
			</div>
		</section>

		<button class="link-button" type="button" onclick={() => (showAdvanced = !showAdvanced)}>
			{showAdvanced ? 'Hide' : 'Show'} advanced custom pull
		</button>
		{#if showAdvanced}
			<div class="advanced-row">
				<input
					class="advanced-input"
					bind:value={customModel}
					placeholder="model:tag (e.g. llama3.2:3b)"
					spellcheck="false"
					disabled={pullingId.startsWith('custom:')}
				/>
				{#if pullingId.startsWith('custom:')}
					{#if pullProgress?.percent != null}
						<span class="settings-field-hint custom-progress"
							>{pullProgress.percent}%</span
						>
					{/if}
					<SettingsButton variant="secondary" onclick={() => void cancelPull()}
						>Cancel</SettingsButton
					>
				{:else}
					<SettingsButton
						variant="secondary"
						disabled={Boolean(pullingId) || !customModel.trim()}
						onclick={() => void pullCustom()}
					>
						Pull custom
					</SettingsButton>
				{/if}
			</div>
		{/if}

		<section class="settings-section installed-section">
			<div class="settings-section-heading">
				<div>
					<h4>Installed models</h4>
					<p>
						Disk usage shown when available. To free space, remove models in Ollama
						(Cometline does not delete shared model files).
					</p>
				</div>
			</div>

			{#if installed.length === 0}
				<p class="settings-field-hint">No models installed yet.</p>
			{:else}
				<div class="settings-scroll-list model-list scrollbar-none">
					{#each installed as model (model.name)}
						{@const limits = modelStore.limitFor(provider.id, model.name)}
						<div class="installed-row">
							<ModelRow
								model={model.name}
								providerId={provider.id}
								enabled={provider.enabledModels.includes(model.name)}
								context={limits?.context}
								inputModalities={limits?.inputModalities}
								modalitiesKnown={limits?.visionKnown}
								onclick={() => toggleModel(model.name)}
							/>
							{#if model.size}
								<small class="size-meta">{formatBytes(model.size)}</small>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</div>

<style>
	.ollama-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-width: 0;
	}

	.field-note {
		display: grid;
		gap: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.55);
		padding: 10px 11px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.field-note > span:first-child {
		font-weight: 700;
		color: var(--text-main);
	}

	.field-note p {
		max-width: 640px;
		font-weight: 500;
		line-height: 1.45;
		margin: 0;
	}

	.field-note.ok .status-line {
		color: var(--status-success);
		font-weight: 650;
	}

	.field-note.bad {
		border-color: var(--status-error-border);
		background: var(--status-error-bg);
	}

	.inline-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding-top: 2px;
	}

	.link-button {
		appearance: none;
		border: none;
		background: transparent;
		padding: 0;
		margin: 0;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
		cursor: pointer;
		text-align: left;
	}

	.link-button:hover {
		text-decoration: underline;
	}

	.guide {
		margin: 0;
		padding-left: 1.15rem;
		display: grid;
		gap: 4px;
	}

	.settings-field-hint.error {
		color: var(--status-error);
	}

	.progress-track {
		height: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
		overflow: hidden;
	}

	.progress-track span {
		display: block;
		height: 100%;
		background: var(--accent);
		border-radius: inherit;
		transition: width 160ms ease;
	}

	.progress-track span.indeterminate {
		opacity: 0.55;
	}

	.catalog-section,
	.installed-section {
		margin-top: 4px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
	}

	.catalog-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.catalog-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 9px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.55);
	}

	.catalog-item.pulling {
		border-color: rgba(0, 102, 204, 0.22);
		background: rgba(0, 102, 204, 0.04);
	}

	.catalog-item-body {
		min-width: 0;
		flex: 1;
	}

	.catalog-item-body strong {
		display: block;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
		line-height: 1.3;
	}

	.catalog-item-body small {
		display: block;
		margin-top: 2px;
		font-size: 10px;
		line-height: 1.35;
		color: var(--text-soft);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta-sep {
		margin: 0 0.25em;
		opacity: 0.7;
	}

	.catalog-item-action {
		flex-shrink: 0;
		display: flex;
		align-items: center;
	}

	.catalog-item-action :global(.catalog-btn) {
		padding: 5px 10px;
		min-width: 4.5rem;
		justify-content: center;
	}

	.pull-inline {
		display: grid;
		gap: 3px;
		margin-top: 6px;
		max-width: 220px;
	}

	.pull-status {
		font-size: 10px;
		color: var(--text-soft);
	}

	.installed-label {
		font-size: 11px;
		font-weight: 600;
		color: var(--status-success);
		padding: 0 4px;
	}

	.size-meta {
		font-size: 11px;
		color: var(--text-muted);
	}

	.custom-progress {
		white-space: nowrap;
	}

	.advanced-row {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}

	.advanced-input {
		flex: 1;
		min-width: 180px;
	}

	.model-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.installed-row {
		display: grid;
		gap: 2px;
	}

	.installed-row .size-meta {
		padding-left: 2px;
	}
</style>
