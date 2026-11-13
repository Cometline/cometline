<script lang="ts">
	import { type WorkspaceContentMatch } from '#lib/client/cometmind.js';
	import { runContentSearch } from '#lib/features/workspace/content-search.js';
	import { openWorkspaceFilePreview } from '#lib/features/workspace/open-file-preview.js';
	import { toWikiUiPath } from '#lib/wiki/paths.js';
	import { normalizeWorkspacePath } from '#lib/features/workspace/file-index.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';

	let source = $state<'workspace' | 'wiki'>('workspace');
	let open = $state(false);
	let query = $state('');
	let matches = $state<WorkspaceContentMatch[]>([]);
	let error = $state('');
	let loading = $state(false);
	let requestId = 0;

	const workspacePath = $derived(normalizeWorkspacePath(shellStore.workspacePath));

	function openModal(dialog: HTMLDialogElement) {
		dialog.showModal();
		return () => dialog.close();
	}

	export function openSearch(next: 'workspace' | 'wiki' = 'workspace') {
		source = next;
		open = true;
	}

	function onSearchRequest(event: Event) {
		const detail = (event as CustomEvent<{ source?: 'workspace' | 'wiki' }>).detail;
		openSearch(detail?.source === 'wiki' ? 'wiki' : 'workspace');
	}

	$effect(() => {
		window.addEventListener('cometline:workspace-content-search', onSearchRequest);
		return () =>
			window.removeEventListener('cometline:workspace-content-search', onSearchRequest);
	});

	async function runSearch() {
		const term = query.trim();
		const current = ++requestId;
		if (!term || (source === 'workspace' && !workspacePath)) {
			matches = [];
			return;
		}
		loading = true;
		error = '';
		try {
			const result = await runContentSearch(source, term, workspacePath);
			if (current !== requestId) return;
			matches = result;
		} catch (cause) {
			if (current !== requestId) return;
			error = cause instanceof Error ? cause.message : 'Search failed';
			matches = [];
		} finally {
			if (current === requestId) loading = false;
		}
	}

	function selectMatch(match: WorkspaceContentMatch) {
		const path = source === 'wiki' ? toWikiUiPath(match.path) : match.path;
		openWorkspaceFilePreview(path, { startLine: match.line, endLine: match.line });
		open = false;
	}
</script>

{#if open}
	<dialog class="content-search-modal" {@attach openModal} aria-labelledby="content-search-title">
		<div class="content-search-panel">
			<h2 id="content-search-title">
				{source === 'wiki' ? 'Search wiki' : 'Search workspace'}
			</h2>
			<input
				bind:value={query}
				placeholder={source === 'wiki'
					? 'Search wiki content…'
					: 'Search workspace content…'}
				aria-label="Search content"
				onkeydown={(event) => {
					if (event.key === 'Enter') void runSearch();
					if (event.key === 'Escape') open = false;
				}}
			/>
			{#if error}
				<p class="content-search-state content-search-error">{error}</p>
			{:else if loading}
				<p class="content-search-state">Searching…</p>
			{:else if matches.length === 0}
				<p class="content-search-state">
					{query.trim() ? 'No matches.' : 'Start typing to search content.'}
				</p>
			{:else}
				<ul>
					{#each matches as match (`${match.path}:${match.line}:${match.column}`)}
						<li>
							<button type="button" onclick={() => selectMatch(match)}>
								<strong>{match.path}:{match.line}</strong>
								<span>{match.preview}</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
			<p class="content-search-hint">Enter search · Esc close</p>
		</div>
	</dialog>
{/if}

<style>
	.content-search-modal {
		position: fixed;
		z-index: 1000;
		top: 18vh;
		left: 50%;
		width: min(560px, calc(100vw - 32px));
		margin: 0;
		padding: 0;
		border: none;
		background: transparent;
		transform: translateX(-50%);
	}

	.content-search-modal::backdrop {
		background: rgba(17, 24, 39, 0.32);
		backdrop-filter: blur(8px);
	}

	.content-search-panel {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 14px;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 16px;
		background: rgba(250, 250, 249, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.18);
	}

	h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
	}

	input {
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 10px;
		padding: 10px 12px;
		font-size: 14px;
	}

	ul {
		max-height: 320px;
		margin: 0;
		padding: 0;
		overflow: auto;
		list-style: none;
	}

	button {
		width: 100%;
		padding: 8px 10px;
		border: none;
		border-radius: 8px;
		background: transparent;
		text-align: left;
		cursor: pointer;
	}

	button:hover {
		background: var(--surface-muted, rgba(0, 0, 0, 0.04));
	}

	.content-search-state,
	.content-search-hint,
	span {
		color: var(--text-muted);
		font-size: 12px;
	}

	.content-search-error {
		color: var(--status-error, var(--color-b91c1c));
	}

	li + li {
		margin-top: 6px;
	}
</style>
