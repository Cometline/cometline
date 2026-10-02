<script lang="ts">
	import { ExternalLink, LoaderCircle, RefreshCw } from '@lucide/svelte';
	import { openOllamaDownloadPage, type OllamaHealthResult } from '#lib/ollama/client.js';
	import SettingsButton from '../SettingsButton.svelte';

	let {
		health,
		checking,
		onRefresh
	}: {
		health: OllamaHealthResult | null;
		checking: boolean;
		onRefresh: () => void;
	} = $props();
</script>

<div class="field-note" class:ok={Boolean(health?.ok)} class:bad={Boolean(health && !health.ok)}>
	<span>Local runtime</span>
	<p class="status-line">
		{#if !health}
			Checking Ollama…
		{:else if health.ok}
			Ollama is ready{health.version ? ` (v${health.version})` : ''}. Connected to
			<code>{health.baseURL}</code>.
		{:else}
			Ollama is not installed or not running. Install Ollama, launch it once, then click Check
			again.
		{/if}
	</p>
	<div class="inline-actions">
		<SettingsButton variant="secondary" disabled={checking} onclick={onRefresh}>
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

<style>
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
</style>
