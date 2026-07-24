<script lang="ts">
	import { tick } from 'svelte';
	import { Loader } from '@lucide/svelte';
	import FileTypeIcon from '$lib/components/FileTypeIcon.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { toWikiUiPath } from '$lib/wiki/paths';
	import {
		loadFileSearchOptions,
		type FileSearchSource
	} from '$lib/workspace/file-search';
	import { normalizeWorkspacePath } from '$lib/workspace/file-index';

	let {
		open = false,
		onClose
	}: {
		open?: boolean;
		onClose: () => void;
	} = $props();

	let query = $state('');
	let debouncedQuery = $state('');
	let results = $state<string[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let activeIndex = $state(0);
	let inputEl = $state<HTMLInputElement | null>(null);
	let resultsListEl = $state<HTMLUListElement | null>(null);
	let loadSeq = 0;
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	const source = $derived(settingsStore.settings.app.fileSearchSource);
	const workspacePath = $derived(normalizeWorkspacePath(shellStore.workspacePath));
	const workspaceAvailable = $derived(Boolean(workspacePath && workspacePath !== '/'));

	function fileName(path: string): string {
		return path.split(/[/\\]/).filter(Boolean).pop() || path;
	}

	function fileDir(path: string): string {
		const parts = path.split(/[/\\]/).filter(Boolean);
		if (parts.length <= 1) return '';
		return parts.slice(0, -1).join('/');
	}

	function openModal(dialog: HTMLDialogElement) {
		dialog.showModal();
		queueMicrotask(() => {
			inputEl?.focus();
			inputEl?.select();
		});
		return () => dialog.close();
	}

	function focusInput(node: HTMLInputElement) {
		inputEl = node;
		return () => {
			if (inputEl === node) inputEl = null;
		};
	}

	function setSource(next: FileSearchSource) {
		if (next === 'workspace' && !workspaceAvailable) return;
		void settingsStore.saveFileSearchSource(next);
	}

	function close() {
		onClose();
	}

	async function selectPath(path: string) {
		const openPath = source === 'wiki' ? toWikiUiPath(path) : path;
		// Older renderer mocks expose this action as void; only an explicit false
		// means the editor's leave guard rejected the requested navigation.
		if ((await shellStore.openFilePreviewForActive(openPath)) !== false) close();
	}

	async function scrollActiveIntoView() {
		const index = activeIndex;
		if (!resultsListEl || results.length === 0) return;
		await tick();
		const el = resultsListEl.querySelector(`[data-result-index="${index}"]`);
		el?.scrollIntoView({ block: 'nearest' });
	}

	function moveActive(delta: number) {
		if (results.length === 0) return;
		activeIndex = (activeIndex + delta + results.length) % results.length;
		void scrollActiveIntoView();
	}

	function onKeydown(event: KeyboardEvent) {
		if (!open) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopImmediatePropagation();
			close();
			return;
		}
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			moveActive(1);
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			moveActive(-1);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			event.stopPropagation();
			const path = results[activeIndex];
			if (path) void selectPath(path);
		}
	}

	function cancelOnBackdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) close();
	}

	async function loadResults() {
		const seq = ++loadSeq;
		loading = true;
		error = null;
		try {
			if (source === 'workspace' && !workspaceAvailable) {
				if (seq !== loadSeq) return;
				results = [];
				return;
			}
			const files = await loadFileSearchOptions(source, workspacePath, debouncedQuery, 50);
			if (seq !== loadSeq) return;
			results = files;
			activeIndex = 0;
		} catch (err) {
			if (seq !== loadSeq) return;
			results = [];
			error = err instanceof Error ? err.message : 'Failed to search files';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	$effect(() => {
		if (!open) return;
		void [source, workspacePath, debouncedQuery];
		void loadResults();
	});

	$effect(() => {
		if (!open) {
			query = '';
			debouncedQuery = '';
			results = [];
			activeIndex = 0;
			error = null;
			return;
		}
		const next = query;
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			debouncedQuery = next;
		}, 120);
		return () => {
			if (debounceTimer) clearTimeout(debounceTimer);
		};
	});
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<dialog
		{@attach openModal}
		class="file-search-modal"
		oncancel={(event) => {
			event.preventDefault();
			close();
		}}
		onclick={cancelOnBackdrop}
		aria-labelledby="file-search-title"
	>
		<div class="file-search-panel">
			<div class="file-search-header">
				<h2 id="file-search-title" class="file-search-title">Search files</h2>
				<div class="source-toggle" role="group" aria-label="File search source">
					<button
						type="button"
						class="source-toggle-btn"
						class:active={source === 'wiki'}
						onclick={() => setSource('wiki')}
					>
						Wiki
					</button>
					<button
						type="button"
						class="source-toggle-btn"
						class:active={source === 'workspace'}
						disabled={!workspaceAvailable}
						title={workspaceAvailable
							? 'Workspace files'
							: 'Select a workspace to search files'}
						onclick={() => setSource('workspace')}
					>
						Workspace
					</button>
				</div>
			</div>

			<input
				{@attach focusInput}
				class="file-search-input"
				type="text"
				placeholder={source === 'wiki' ? 'Search wiki files…' : 'Search workspace files…'}
				bind:value={query}
				aria-label="Search files"
				autocomplete="off"
				spellcheck="false"
			/>

			{#if error}
				<div class="file-search-state file-search-error">{error}</div>
			{:else if loading && results.length === 0}
				<div class="file-search-state">
					<Loader size={16} stroke-width={2} class="file-search-spinner" />
					<span>Searching…</span>
				</div>
			{:else if source === 'workspace' && !workspaceAvailable}
				<div class="file-search-state">Select a workspace to search its files.</div>
			{:else if results.length === 0}
				<div class="file-search-state">
					{debouncedQuery.trim() ? 'No matching files.' : 'No files found.'}
				</div>
			{:else}
				<ul
					class="file-search-list"
					role="listbox"
					aria-label="File results"
					bind:this={resultsListEl}
				>
					{#each results as path, index (path)}
						<li role="option" aria-selected={index === activeIndex}>
							<button
								type="button"
								class="file-search-row"
								class:active={index === activeIndex}
								data-result-index={index}
								onmouseenter={() => (activeIndex = index)}
								onclick={() => void selectPath(path)}
								title={path}
							>
								<span class="file-search-icon" aria-hidden="true">
									<FileTypeIcon {path} size={16} />
								</span>
								<span class="file-search-labels">
									<span class="file-search-name">{fileName(path)}</span>
									{#if fileDir(path)}
										<span class="file-search-dir">{fileDir(path)}</span>
									{/if}
								</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}

			<p class="file-search-hint">
				↑↓ to navigate · Enter to open · Esc to close
			</p>
		</div>
	</dialog>
{/if}

<style>
	.file-search-modal {
		position: fixed;
		top: 18vh;
		left: 50%;
		margin: 0;
		transform: translateX(-50%);
		width: min(560px, calc(100vw - 32px));
		padding: 0;
		border: none;
		background: transparent;
		overflow: visible;
	}

	.file-search-modal::backdrop {
		background: rgba(17, 24, 39, 0.32);
		backdrop-filter: blur(8px);
	}

	.file-search-panel {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 14px;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 16px;
		background: rgba(250, 250, 249, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.18);
	}

	.file-search-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.file-search-title {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
		color: var(--text-main, #111);
	}

	.source-toggle {
		display: flex;
		gap: 2px;
		padding: 2px;
		border-radius: 8px;
		background: var(--surface-muted, rgba(0, 0, 0, 0.04));
	}

	.source-toggle-btn {
		border: none;
		border-radius: 6px;
		padding: 5px 10px;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 550;
		cursor: pointer;
	}

	.source-toggle-btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.source-toggle-btn.active {
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
		box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.06);
	}

	.file-search-input {
		width: 100%;
		box-sizing: border-box;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 10px;
		padding: 10px 12px;
		font-size: 14px;
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
	}

	.file-search-input:focus {
		outline: none;
		border-color: var(--accent, #3b82f6);
	}

	.file-search-list {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: min(48vh, 360px);
		overflow: auto;
	}

	.file-search-row {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		border: none;
		border-radius: 8px;
		padding: 8px 10px;
		background: transparent;
		color: var(--text-primary, #111);
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}

	.file-search-row:hover,
	.file-search-row.active {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, #9a9a9f) 14%,
			transparent
		);
	}

	.file-search-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 16px;
		width: 16px;
		height: 16px;
	}

	.file-search-labels {
		display: flex;
		align-items: baseline;
		gap: 8px;
		min-width: 0;
		flex: 1 1 auto;
	}

	.file-search-name {
		flex: 0 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 550;
	}

	.file-search-dir {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
		font-size: 12px;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	.file-search-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 88px;
		padding: 16px;
		color: var(--text-muted);
		font-size: 13px;
		text-align: center;
	}

	.file-search-error {
		color: var(--status-error, #b91c1c);
	}

	.file-search-state :global(.file-search-spinner) {
		animation: file-search-spin 0.7s linear infinite;
	}

	.file-search-hint {
		margin: 0;
		padding: 0 2px;
		color: var(--text-soft, #9a9a9f);
		font-size: 11px;
	}

	@keyframes file-search-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.file-search-state :global(.file-search-spinner) {
			animation: none;
		}
	}
</style>
