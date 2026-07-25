<script lang="ts">
	import { Loader } from '@lucide/svelte';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import FileEditor from '$lib/components/FileEditor.svelte';
	import {
		listWikiFileBacklinks,
		readWikiFileContent,
		readWorkspaceFileContent,
		writeWikiFileContent,
		writeWorkspaceFileContent
	} from '$lib/client/cometmind';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { openWorkspaceFilePreview } from '$lib/workspace/open-file-preview';
	import {
		isMarkdownPath,
		languageFromExtension,
		languageFromPath
	} from '$lib/workspace/file-preview';
	import {
		buildFileSnippetContext,
		type SelectionLineRange
	} from '$lib/workspace/selection-snippet';
	import type { FileRevealRange } from '$lib/workspace/workspace-panel-state';
	import {
		readMarkdownFileViewMode,
		writeMarkdownFileViewMode,
		type MarkdownFileViewMode
	} from '$lib/workspace/workspace-panel-prefs';
	import { refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';
	import { isWikiReadOnlyPath, isWikiUiPath, toWikiRelative, toWikiUiPath } from '$lib/wiki/paths';
	import { wikiStemFromPath } from '$lib/wiki/wikilinks';

	type EditorState = {
		dirty: boolean;
		saving: boolean;
		saveError: string | null;
		save: () => Promise<void>;
		revert: () => void;
	};

	let {
		workspacePath,
		filePath,
		revealRange = null,
		onEditorState
	}: {
		workspacePath: string;
		filePath: string;
		revealRange?: FileRevealRange | null;
		onEditorState?: (state: EditorState | null) => void;
	} = $props();

	let loading = $state(true);
	let error = $state<string | null>(null);
	let imageDataUrl = $state('');
	let savedContent = $state('');
	let draftContent = $state('');
	let language = $state<string | null>(null);
	let previewKind = $state<'text' | 'image' | null>(null);
	let saving = $state(false);
	let saveError = $state<string | null>(null);
	let loadVersion = 0;
	let viewMode = $state<MarkdownFileViewMode>(readMarkdownFileViewMode());
	let wikiFiles = $state<string[]>([]);
	let backlinks = $state<string[]>([]);
	let backlinksLoading = $state(false);
	let fileEditor = $state<{
		getSelectionRange: () => {
			text: string;
			startLine: number;
			endLine: number;
		} | null;
	} | null>(null);
	let selectionPopup = $state<{
		top: number;
		left: number;
		text: string;
		lineRange: SelectionLineRange | null;
	} | null>(null);

	const readOnly = $derived(isWikiUiPath(filePath) && isWikiReadOnlyPath(filePath));
	const dirty = $derived(previewKind === 'text' && draftContent !== savedContent && !readOnly);
	const isMarkdown = $derived(isMarkdownPath(filePath));
	const showMarkdownToggle = $derived(previewKind === 'text' && isMarkdown);
	const effectiveViewMode = $derived(
		showMarkdownToggle ? viewMode : ('source' satisfies MarkdownFileViewMode)
	);
	const isWikiFile = $derived(isWikiUiPath(filePath));
	const showBacklinks = $derived(
		isWikiFile && previewKind === 'text' && !loading && !error
	);

	function setViewMode(mode: MarkdownFileViewMode) {
		viewMode = mode;
		writeMarkdownFileViewMode(mode);
		selectionPopup = null;
	}

	function clearSelectionPopup() {
		selectionPopup = null;
	}

	function placeSelectionPopup(
		text: string,
		clientRect: DOMRect,
		lineRange: SelectionLineRange | null
	) {
		if (!text.trim()) {
			clearSelectionPopup();
			return;
		}
		selectionPopup = {
			top: clientRect.bottom + 8,
			left: Math.min(Math.max(8, clientRect.left), window.innerWidth - 140),
			text,
			lineRange
		};
	}

	function onPreviewMouseUp(event: MouseEvent) {
		const root = event.currentTarget;
		if (!(root instanceof HTMLElement)) return;
		const sel = window.getSelection();
		if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
			clearSelectionPopup();
			return;
		}
		if (!root.contains(sel.anchorNode) || !root.contains(sel.focusNode)) {
			clearSelectionPopup();
			return;
		}
		const text = sel.toString();
		placeSelectionPopup(text, sel.getRangeAt(0).getBoundingClientRect(), null);
	}

	function onSourceMouseUp() {
		const selected = fileEditor?.getSelectionRange() ?? null;
		if (!selected) {
			clearSelectionPopup();
			return;
		}
		const sel = window.getSelection();
		const rect =
			sel && sel.rangeCount > 0 && !sel.isCollapsed
				? sel.getRangeAt(0).getBoundingClientRect()
				: new DOMRect(24, 72, 0, 0);
		placeSelectionPopup(selected.text, rect, {
			startLine: selected.startLine,
			endLine: selected.endLine
		});
	}

	function addSelectionToChat() {
		if (!selectionPopup) return;
		const ctx = buildFileSnippetContext({
			filePath,
			selectedText: selectionPopup.text,
			sourceText: draftContent,
			lineRange: selectionPopup.lineRange
		});
		if (ctx) {
			shellStore.addWebContextForActive(ctx);
			shellStore.requestComposerFocus();
		}
		clearSelectionPopup();
		window.getSelection()?.removeAllRanges();
	}

	function revert() {
		if (previewKind !== 'text') return;
		draftContent = savedContent;
		saveError = null;
	}

	async function save() {
		if (previewKind !== 'text' || saving || !dirty || readOnly) return;

		const nextContent = draftContent;
		const currentWorkspacePath = workspacePath;
		const currentFilePath = filePath;

		saving = true;
		saveError = null;
		try {
			if (isWikiUiPath(currentFilePath)) {
				await writeWikiFileContent(toWikiRelative(currentFilePath), nextContent);
			} else {
				await writeWorkspaceFileContent(currentWorkspacePath, currentFilePath, nextContent);
			}
			if (workspacePath !== currentWorkspacePath || filePath !== currentFilePath) return;
			savedContent = nextContent;
			draftContent = nextContent;
			if (isWikiUiPath(currentFilePath)) {
				void refreshWikiFileIndex(true);
				void loadBacklinks(currentFilePath);
			}
		} catch (err) {
			if (workspacePath !== currentWorkspacePath || filePath !== currentFilePath) return;
			saveError = err instanceof Error ? err.message : 'Failed to save file';
		} finally {
			if (workspacePath === currentWorkspacePath && filePath === currentFilePath) {
				saving = false;
			}
		}
	}

	async function loadBacklinks(path: string) {
		if (!isWikiUiPath(path)) {
			backlinks = [];
			return;
		}
		backlinksLoading = true;
		try {
			backlinks = await listWikiFileBacklinks(toWikiRelative(path));
		} catch {
			backlinks = [];
		} finally {
			backlinksLoading = false;
		}
	}

	async function loadPreview() {
		const version = ++loadVersion;
		loading = true;
		error = null;
		imageDataUrl = '';
		savedContent = '';
		draftContent = '';
		language = null;
		previewKind = null;
		saving = false;
		saveError = null;
		backlinks = [];
		selectionPopup = null;

		try {
			const wikiIndexPromise = refreshWikiFileIndex(true);
			const result = isWikiUiPath(filePath)
				? await readWikiFileContent(toWikiRelative(filePath))
				: await readWorkspaceFileContent(workspacePath, filePath);
			if (version !== loadVersion) return;

			wikiFiles = await wikiIndexPromise;
			if (version !== loadVersion) return;

			if (result.kind === 'image') {
				previewKind = 'image';
				imageDataUrl = result.data_url;
				return;
			}

			savedContent = result.content;
			draftContent = result.content;
			language = languageFromPath(filePath) ?? languageFromExtension(result.extension);
			previewKind = 'text';
			void loadBacklinks(filePath);
		} catch (err) {
			if (version !== loadVersion) return;
			error = err instanceof Error ? err.message : 'Failed to load file';
		} finally {
			if (version === loadVersion) loading = false;
		}
	}

	$effect(() => {
		// Track both inputs so the editor reloads when either changes.
		void [workspacePath, filePath];
		void loadPreview();
	});

	$effect(() => {
		// Jumping to a line range requires the source editor, not markdown preview.
		if (revealRange && isMarkdown && viewMode === 'preview') {
			viewMode = 'source';
			writeMarkdownFileViewMode('source');
		}
	});

	$effect(() => {
		onEditorState?.(
			previewKind === 'text' && !loading && !error && !readOnly
				? {
						dirty,
						saving,
						saveError,
						save,
						revert
					}
				: null
		);
	});

	$effect(() => {
		return () => {
			onEditorState?.(null);
		};
	});
</script>

{#snippet backlinksSection()}
	<section class="backlinks" aria-label="Backlinks">
		<h3 class="backlinks-title">Backlinks</h3>
		{#if backlinksLoading}
			<p class="backlinks-empty">Loading backlinks…</p>
		{:else if backlinks.length === 0}
			<p class="backlinks-empty">No backlinks yet.</p>
		{:else}
			<ul class="backlinks-list">
				{#each backlinks as linkPath (linkPath)}
					<li>
						<button
							type="button"
							class="backlink-item"
							onclick={() => openWorkspaceFilePreview(toWikiUiPath(linkPath))}
						>
							{wikiStemFromPath(linkPath)}
							<span class="backlink-path">{linkPath}</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/snippet}

<div class="file-preview scrollbar-none" aria-live="polite">
	{#if loading}
		<div class="file-preview-state">
			<Loader size={16} stroke-width={2} class="file-preview-spinner" />
			<span>Loading file…</span>
		</div>
	{:else if error}
		<div class="file-preview-state file-preview-error">{error}</div>
	{:else if previewKind === 'image'}
		<div class="file-preview-image-wrap">
			<img src={imageDataUrl} alt={filePath} class="file-preview-image" />
		</div>
	{:else if previewKind === 'text'}
		<div class="file-preview-editor-wrap">
			{#if showMarkdownToggle}
				<div class="md-view-toggle" role="group" aria-label="Markdown view mode">
					<button
						type="button"
						class="md-view-toggle-btn"
						class:active={effectiveViewMode === 'preview'}
						onclick={() => setViewMode('preview')}
					>
						Preview
					</button>
					<button
						type="button"
						class="md-view-toggle-btn"
						class:active={effectiveViewMode === 'source'}
						onclick={() => setViewMode('source')}
					>
						Source
					</button>
				</div>
			{/if}
			{#if saveError}
				<div class="file-preview-save-error">{saveError}</div>
			{/if}
			{#if effectiveViewMode === 'preview'}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="file-preview-markdown scrollbar-none"
					onmouseup={onPreviewMouseUp}
				>
					<AssistantMarkdown
						source={draftContent}
						mode="assistant"
						{wikiFiles}
						workspaceResources={{
							kind: isWikiFile ? 'wiki' : 'workspace',
							workspacePath,
							filePath: isWikiFile ? toWikiRelative(filePath) : filePath,
							readFile: (relativePath) =>
								isWikiFile
									? readWikiFileContent(relativePath)
									: readWorkspaceFileContent(workspacePath, relativePath)
						}}
					/>
					{#if showBacklinks}
						{@render backlinksSection()}
					{/if}
				</div>
			{:else}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="file-preview-source-wrap" onmouseup={onSourceMouseUp}>
					<FileEditor
						bind:this={fileEditor}
						value={draftContent}
						{language}
						readOnly={saving || readOnly}
						{revealRange}
						onChange={(value) => {
							draftContent = value;
							if (saveError) saveError = null;
						}}
						onSave={() => {
							void save();
						}}
						onRevealApplied={() => shellStore.clearFileRevealForActive()}
					/>
					{#if showBacklinks && !showMarkdownToggle}
						{@render backlinksSection()}
					{/if}
				</div>
			{/if}
		</div>
	{/if}

	{#if selectionPopup}
		<button
			type="button"
			class="selection-add-chat"
			style:top="{selectionPopup.top}px"
			style:left="{selectionPopup.left}px"
			onmousedown={(event) => event.preventDefault()}
			onclick={addSelectionToChat}
		>
			Add to chat
		</button>
	{/if}
</div>

<style>
	.file-preview {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: auto;
		background: #fff;
	}

	.selection-add-chat {
		position: fixed;
		z-index: 40;
		padding: 6px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: #fff;
		color: var(--text-main);
		font-size: 12px;
		font-weight: 600;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
		cursor: pointer;
	}

	.selection-add-chat:hover {
		border-color: var(--text-soft);
	}

	.file-preview-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 120px;
		padding: 24px;
		color: var(--text-muted);
		font-size: 13px;
	}

	.file-preview-error {
		color: var(--status-error);
		text-align: center;
	}

	.file-preview-state :global(.file-preview-spinner) {
		animation: file-preview-spin 0.7s linear infinite;
	}

	@keyframes file-preview-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.file-preview-image-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100%;
		padding: 16px;
		box-sizing: border-box;
	}

	.file-preview-image {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain;
	}

	.file-preview-editor-wrap {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
	}

	.md-view-toggle {
		display: flex;
		gap: 2px;
		flex: 0 0 auto;
		padding: 8px 10px;
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
		background: rgba(0, 0, 0, 0.02);
	}

	.md-view-toggle-btn {
		border: none;
		border-radius: 6px;
		padding: 5px 10px;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 550;
		cursor: pointer;
	}

	.md-view-toggle-btn.active {
		background: #fff;
		color: var(--text-primary, #111);
		box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.06);
	}

	.file-preview-markdown {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 16px 18px 24px;
		box-sizing: border-box;
	}

	.file-preview-source-wrap {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
	}

	.file-preview-source-wrap :global(.file-editor) {
		flex: 1;
		min-height: 0;
	}

	.file-preview-source-wrap .backlinks {
		flex: 0 0 auto;
		padding: 0 18px 24px;
	}

	.backlinks {
		margin-top: 28px;
		padding-top: 16px;
		border-top: 1px solid rgba(0, 0, 0, 0.08);
	}

	.backlinks-title {
		margin: 0 0 10px;
		font-size: 12px;
		font-weight: 650;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--text-muted);
	}

	.backlinks-empty {
		margin: 0;
		font-size: 13px;
		color: var(--text-muted);
	}

	.backlinks-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.backlink-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		width: 100%;
		border: none;
		border-radius: 8px;
		padding: 8px 10px;
		background: rgba(0, 0, 0, 0.02);
		color: var(--text-primary, #111);
		font-size: 13px;
		font-weight: 550;
		text-align: left;
		cursor: pointer;
	}

	.backlink-item:hover {
		background: rgba(0, 0, 0, 0.05);
	}

	.backlink-path {
		font-size: 11px;
		font-weight: 450;
		color: var(--text-muted);
	}

	.file-preview-save-error {
		padding: 10px 14px;
		border-bottom: 1px solid rgba(180, 35, 24, 0.15);
		background: rgba(180, 35, 24, 0.05);
		color: var(--status-error);
		font-size: 12px;
	}

	@media (prefers-reduced-motion: reduce) {
		.file-preview-state :global(.file-preview-spinner) {
			animation: none;
		}
	}
</style>
