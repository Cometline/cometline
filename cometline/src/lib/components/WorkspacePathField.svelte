<script lang="ts">
	import { ChevronDown, FolderOpen } from '@lucide/svelte';
	import { filterWorkspaceOptions } from '$lib/skills/slash-commands';
	import { loadWorkspacePaths } from '$lib/workspaces/load-workspace-paths';
	import { shellStore } from '$lib/stores/shell.svelte';

	let {
		value = $bindable(''),
		placeholder = 'Select a workspace…',
		disabled = false,
		showCurrentShortcut = true
	}: {
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		showCurrentShortcut?: boolean;
	} = $props();

	let panelOpen = $state(false);
	let loading = $state(false);
	let loaded = $state(false);
	let searchQuery = $state('');
	let workspacePaths = $state<string[]>([]);
	let sessionCountByPath = $state(new Map<string, number>());

	const canBrowse = $derived(Boolean(window.electronAPI?.selectWorkspacePath));
	const filteredOptions = $derived(
		filterWorkspaceOptions(searchQuery, workspacePaths, sessionCountByPath)
	);
	const currentWorkspace = $derived(shellStore.workspacePath?.trim() ?? '');

	async function ensurePathsLoaded() {
		if (loaded || loading) return;
		loading = true;
		try {
			const result = await loadWorkspacePaths(currentWorkspace || value.trim() || undefined);
			workspacePaths = result.paths;
			sessionCountByPath = result.sessionCountByPath;
			loaded = true;
		} catch {
			workspacePaths = currentWorkspace ? [currentWorkspace] : [];
			sessionCountByPath = new Map();
			loaded = true;
		} finally {
			loading = false;
		}
	}

	async function togglePanel() {
		if (disabled) return;
		panelOpen = !panelOpen;
		if (panelOpen) await ensurePathsLoaded();
	}

	async function browseFolder() {
		if (disabled) return;
		const picked = await window.electronAPI?.selectWorkspacePath?.();
		if (!picked) return;
		value = picked;
		panelOpen = false;
	}

	function selectPath(path: string) {
		value = path;
		panelOpen = false;
		searchQuery = '';
	}

	function useCurrentWorkspace() {
		if (!currentWorkspace || disabled) return;
		value = currentWorkspace;
	}

	function handlePanelKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			panelOpen = false;
		}
	}
</script>

<div class="workspace-path-field settings-ui">
	<div class="path-row">
		<input
			type="text"
			value={value}
			readonly
			{placeholder}
			disabled={disabled}
			spellcheck="false"
			class="path-input"
			aria-readonly="true"
			onclick={() => void togglePanel()}
		/>
		{#if canBrowse}
			<button type="button" class="secondary" {disabled} onclick={() => void browseFolder()}>
				Browse
			</button>
		{/if}
		<button
			type="button"
			class="secondary icon-only"
			{disabled}
			aria-expanded={panelOpen}
			aria-label={panelOpen ? 'Hide workspaces' : 'Show workspaces'}
			onclick={() => void togglePanel()}
		>
			<span class="chevron" class:expanded={panelOpen}>
				<ChevronDown size={14} />
			</span>
		</button>
	</div>

	{#if showCurrentShortcut && currentWorkspace}
		<div class="workspace-shortcuts">
			<button type="button" class="current-workspace-link" {disabled} onclick={useCurrentWorkspace}>
				Use current workspace
			</button>
			{#if value.trim()}
				<button
					type="button"
					class="current-workspace-link clear"
					{disabled}
					onclick={() => {
						value = '';
						panelOpen = false;
					}}
				>
					Clear
				</button>
			{/if}
		</div>
	{:else if value.trim()}
		<button
			type="button"
			class="current-workspace-link clear"
			{disabled}
			onclick={() => {
				value = '';
				panelOpen = false;
			}}
		>
			Clear
		</button>
	{/if}

	{#if panelOpen}
		<div
			class="workspace-panel scrollbar-none"
			role="listbox"
			tabindex="-1"
			aria-label="Available workspaces"
			onkeydown={handlePanelKeydown}
		>
			<input
				type="search"
				class="workspace-search"
				placeholder="Filter workspaces…"
				bind:value={searchQuery}
				{disabled}
			/>
			{#if loading && !loaded}
				<p class="workspace-panel-empty">Loading workspaces…</p>
			{:else if filteredOptions.length === 0}
				<p class="workspace-panel-empty">No matching workspaces.</p>
			{:else}
				{#each filteredOptions as option (`${option.kind}:${option.path}:${option.label}`)}
					{#if option.kind === 'workspace'}
						<button
							type="button"
							class="workspace-option"
							role="option"
							aria-selected={value === option.path}
							onclick={() => selectPath(option.path)}
						>
							<span class="workspace-option-label">{option.label}</span>
							<span class="workspace-option-path">{option.description}</span>
						</button>
					{:else}
						<button
							type="button"
							class="workspace-option browse"
							role="option"
							aria-selected={false}
							onclick={() => void browseFolder()}
						>
							<FolderOpen size={13} stroke-width={2} />
							<span class="workspace-option-label">{option.label}</span>
							<span class="workspace-option-path">{option.description}</span>
						</button>
					{/if}
				{/each}
			{/if}
		</div>
	{/if}
</div>

<style>
	.workspace-path-field {
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-width: 0;
	}

	.path-row {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.path-input {
		flex: 1;
		min-width: 0;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 10px;
		font: inherit;
		font-size: 12px;
		background: var(--app-bg);
		color: var(--text-main);
		cursor: pointer;
	}

	.path-input:read-only {
		color: var(--text-muted);
		background: rgba(15, 23, 42, 0.04);
	}

	.path-input:read-only:not(:disabled):hover {
		border-color: color-mix(in srgb, var(--accent) 24%, var(--border-soft));
	}

	.path-input:focus {
		outline: none;
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	.path-input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.chevron {
		display: grid;
		place-items: center;
		transition: transform 140ms ease;
	}

	.chevron.expanded {
		transform: rotate(180deg);
	}

	.workspace-shortcuts {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.current-workspace-link {
		align-self: flex-start;
		border: none;
		background: transparent;
		padding: 0;
		font-size: 11px;
		font-weight: 600;
		color: var(--accent);
		cursor: pointer;
	}

	.current-workspace-link.clear {
		color: var(--text-muted);
	}

	.current-workspace-link:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.workspace-panel {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 10px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: var(--panel-bg);
		max-height: 220px;
		overflow-y: auto;
	}

	.workspace-search {
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		padding: 6px 8px;
		font: inherit;
		font-size: 12px;
		background: var(--app-bg);
		color: var(--text-main);
	}

	.workspace-panel-empty {
		margin: 0;
		padding: 8px 4px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.workspace-option {
		display: grid;
		gap: 2px;
		width: 100%;
		text-align: left;
		border: 1px solid transparent;
		border-radius: 8px;
		padding: 8px 10px;
		background: transparent;
		cursor: pointer;
		transition: background 140ms ease, border-color 140ms ease;
	}

	.workspace-option:hover {
		background: rgba(15, 23, 42, 0.05);
		border-color: var(--border-soft);
	}

	.workspace-option.browse {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.workspace-option-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	.workspace-option-path {
		font-size: 11px;
		color: var(--text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
