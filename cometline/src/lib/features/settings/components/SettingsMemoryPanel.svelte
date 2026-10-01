<script lang="ts">
	import type { MemorySettings } from '$lib/client/cometmind';
	import type { SavedEmbeddingRef } from '$lib/embedding-models';
	import SettingsMemoryLibrary from './memory/SettingsMemoryLibrary.svelte';
	import SettingsMemoryRetrieval from './memory/SettingsMemoryRetrieval.svelte';
	import { createSettingsMemoryPanel } from '$lib/features/settings/settings-memory-panel.svelte';
	import type { ProviderConfig } from '$lib/types';

	interface Props {
		providers?: ProviderConfig[];
		savedEmbedding?: SavedEmbeddingRef;
		onEmbeddingSaved?: (embedding: MemorySettings['embedding']) => void | Promise<void>;
	}

	let { providers = [], savedEmbedding, onEmbeddingSaved }: Props = $props();

	const panel = createSettingsMemoryPanel({
		getProviders: () => providers,
		getSavedEmbedding: () => savedEmbedding,
		onEmbeddingSaved: (embedding) => onEmbeddingSaved?.(embedding)
	});

	export function isBusy(): boolean {
		return panel.isBusy();
	}

	export function applySavedMemory(next: MemorySettings) {
		panel.applySavedMemory(next);
	}

	export function isDirty(): boolean {
		return panel.isDirty();
	}

	export function buildSavePayload(): MemorySettings {
		return panel.buildSavePayload();
	}

	export function syncFields() {
		panel.syncFields();
	}

	export async function saveMemorySettings() {
		await panel.saveMemorySettings();
	}
</script>

{#if panel.s.loading}
	<p class="muted">Loading memory settings…</p>
{:else if panel.s.settings}
	<section class="memory-panel settings-panel-frame">
		<div class="settings-panel-body">
			{#if panel.s.loadError}
				<p class="load-error">
					{panel.s.loadError}. Showing defaults — reload CometMind (Save in Settings →
					Providers) or run
					<code>make build-cometmind</code> if endpoints are missing.
					<button class="link-button" type="button" onclick={panel.reload}>Retry</button>
				</p>
			{/if}
			<SettingsMemoryRetrieval {panel} />
			<SettingsMemoryLibrary {panel} />
		</div>
	</section>
{:else}
	<p class="load-error">
		{panel.s.loadError || 'Could not load memory settings.'}
		<button class="link-button" type="button" onclick={panel.reload}>Retry</button>
	</p>
{/if}

<style>
	.muted,
	.load-error {
		font-size: 12px;
		color: var(--text-muted);
	}

	.load-error {
		padding: 12px;
		border: 1px solid rgba(180, 35, 24, 0.25);
		border-radius: 12px;
		background: rgba(180, 35, 24, 0.06);
		color: var(--status-error);
	}

	.link-button {
		border: none;
		background: none;
		padding: 0;
		margin-left: 6px;
		font: inherit;
		font-size: inherit;
		color: var(--accent);
		cursor: pointer;
		text-decoration: underline;
	}
</style>
