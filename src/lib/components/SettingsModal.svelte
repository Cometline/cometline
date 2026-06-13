<script lang="ts">
	import { fly, fade, scale } from 'svelte/transition';
	import { Check, Loader2, Settings, X } from '@lucide/svelte';
	import type { ProviderSettings } from '$lib/types';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';

	let draft = $state<ProviderSettings>({ ...settingsStore.settings, models: [...settingsStore.settings.models] });
	let status = $state('');
	let wasOpen = false;

	$effect(() => {
		const open = shellStore.settingsOpen;
		if (open && !wasOpen) {
			draft = { ...settingsStore.settings, models: [...settingsStore.settings.models] };
			status = '';
		}
		wasOpen = open;
	});

	async function fetchModels() {
		status = '';
		const next = await settingsStore.fetchModels(draft);
		draft = { ...next, models: [...next.models] };
		status = `Fetched ${next.models.length} model${next.models.length === 1 ? '' : 's'}.`;
	}

	async function save() {
		status = '';
		let next = draft;
		if (draft.baseURL.trim() && draft.apiKey.trim()) {
			next = await settingsStore.fetchModels(draft);
		}
		const saved = await settingsStore.save(next);
		draft = { ...saved, models: [...saved.models] };
		status = 'Saved. CometMind is restarting with this provider.';
	}

	function selectModel(model: string) {
		draft = { ...draft, selectedModel: model };
	}
</script>

{#if shellStore.settingsOpen}
	<div class="settings-layer" transition:fade={{ duration: 120 }}>
		<button class="scrim" aria-label="Close settings" onclick={shellStore.closeSettings}></button>
		<div class="modal" role="dialog" aria-modal="true" aria-labelledby="settings-title" transition:scale={{ start: 0.97, duration: 140 }}>
			<header>
				<div class="title-mark"><Settings size={16} /></div>
				<div>
					<h2 id="settings-title">Provider Settings</h2>
					<p>Configure the provider used by new CometMind sessions.</p>
				</div>
				<button class="icon-button" aria-label="Close settings" onclick={shellStore.closeSettings}>
					<X size={16} />
				</button>
			</header>

			<div class="form-grid">
				<label>
					<span>Provider</span>
					<select bind:value={draft.provider}>
						<option value="openai">OpenAI-compatible</option>
						<option value="anthropic">Anthropic</option>
					</select>
				</label>

				<label>
					<span>Base URL</span>
					<input bind:value={draft.baseURL} placeholder="https://example.com/v1" spellcheck="false" />
				</label>

				<label>
					<span>API Key</span>
					<input bind:value={draft.apiKey} type="password" placeholder="sk-..." spellcheck="false" />
				</label>
			</div>

			<div class="model-section">
				<div class="model-heading">
					<div>
						<h3>Available Models</h3>
						<p>Fetched from <code>/models</code> using the Base URL above.</p>
					</div>
					<button class="secondary" onclick={fetchModels} disabled={settingsStore.isFetchingModels || !draft.baseURL.trim() || !draft.apiKey.trim()}>
						{#if settingsStore.isFetchingModels}<Loader2 size={14} class="spin" />{/if}
						Fetch models
					</button>
				</div>

				<div class="models">
					{#each draft.models as model (model)}
						<button class="model-row" class:selected={model === draft.selectedModel} onclick={() => selectModel(model)} transition:fly={{ y: 4, duration: 100 }}>
							<span>{model}</span>
							{#if model === draft.selectedModel}<Check size={14} />{/if}
						</button>
					{:else}
						<p class="empty-models">No models loaded yet.</p>
					{/each}
				</div>
			</div>

			{#if settingsStore.error}
				<p class="message error">{settingsStore.error}</p>
			{:else if status}
				<p class="message success">{status}</p>
			{/if}

			<footer>
				<button class="secondary" onclick={shellStore.closeSettings}>Cancel</button>
				<button class="primary" onclick={save} disabled={settingsStore.isSaving || settingsStore.isFetchingModels || !draft.selectedModel.trim()}>
					{#if settingsStore.isSaving}<Loader2 size={14} class="spin" />{/if}
					Save and restart
				</button>
			</footer>
		</div>
	</div>
{/if}

<style>
	.settings-layer {
		position: absolute;
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
		width: min(620px, 100%);
		max-height: min(720px, 100%);
		overflow: auto;
		background: rgba(255, 255, 255, 0.96);
		border: 1px solid rgba(229, 231, 235, 0.95);
		border-radius: 22px;
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		padding: 18px;
	}

	header,
	footer,
	.model-heading,
	.model-row {
		display: flex;
		align-items: center;
	}

	header {
		gap: 12px;
		padding-bottom: 16px;
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
	}

	header h2,
	header p,
	.model-heading h3,
	.model-heading p,
	.message {
		margin: 0;
	}

	header h2 {
		font-size: 16px;
		font-weight: 650;
	}

	header p,
	.model-heading p,
	.empty-models {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
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
	}

	.icon-button:hover,
	.secondary:hover,
	.model-row:hover {
		background: rgba(15, 23, 42, 0.05);
	}

	.form-grid {
		display: grid;
		gap: 12px;
		padding: 16px 0;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
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

	input:focus,
	select:focus {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	.model-section {
		border: 1px solid var(--border-soft);
		border-radius: 16px;
		padding: 12px;
		background: rgba(251, 251, 250, 0.72);
	}

	.model-heading {
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 10px;
	}

	.model-heading h3 {
		font-size: 13px;
		font-weight: 650;
	}

	.models {
		display: grid;
		gap: 4px;
		max-height: 180px;
		overflow: auto;
	}

	.model-row {
		justify-content: space-between;
		gap: 12px;
		width: 100%;
		border: none;
		border-radius: 10px;
		background: transparent;
		padding: 8px 9px;
		font-size: 12px;
		color: var(--text-main);
		text-align: left;
	}

	.model-row.selected {
		background: rgba(0, 102, 204, 0.08);
		color: var(--accent);
	}

	.secondary,
	.primary {
		border: none;
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 7px;
	}

	.secondary {
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-main);
	}

	.primary {
		background: var(--text-main);
		color: white;
	}

	button:disabled {
		opacity: 0.45;
	}

	.message {
		padding: 10px 2px 0;
		font-size: 12px;
	}

	.message.error {
		color: #b42318;
	}

	.message.success {
		color: #027a48;
	}

	footer {
		justify-content: flex-end;
		gap: 8px;
		padding-top: 16px;
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
