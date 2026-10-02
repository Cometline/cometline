<script lang="ts">
	import { Loader } from '@lucide/svelte';
	import AssistantMarkdown from '#lib/components/AssistantMarkdown.svelte';
	import FileEditor from '#lib/features/workspace/components/FileEditor.svelte';
	import FilePreviewBacklinks from '#lib/features/workspace/components/file-preview/FilePreviewBacklinks.svelte';
	import FilePreviewExternal from '#lib/features/workspace/components/file-preview/FilePreviewExternal.svelte';
	import PdfPreview from '#lib/features/workspace/components/PdfPreview.svelte';
	import SelectionAddToChat from '#lib/components/SelectionAddToChat.svelte';
	import { readWikiFileContent, readWorkspaceFileContent } from '#lib/client/cometmind.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import {
		createFilePreviewController,
		type FilePreviewEditorState
	} from '#lib/features/workspace/file-preview-controller.svelte.js';
	import type { FileRevealRange } from '#lib/features/workspace/workspace-panel-state.js';
	import { toWikiRelative } from '#lib/wiki/paths.js';

	let {
		workspacePath,
		filePath,
		revealRange = null,
		onEditorState
	}: {
		workspacePath: string;
		filePath: string;
		revealRange?: FileRevealRange | null;
		onEditorState?: (state: FilePreviewEditorState | null) => void;
	} = $props();

	const panel = createFilePreviewController({
		getWorkspacePath: () => workspacePath,
		getFilePath: () => filePath,
		getRevealRange: () => revealRange,
		onEditorState: (state) => onEditorState?.(state)
	});
</script>

<div class="file-preview scrollbar-none" aria-live="polite">
	{#if panel.s.loading}
		<div class="file-preview-state">
			<Loader size={16} stroke-width={2} class="file-preview-spinner" />
			<span>Loading file…</span>
		</div>
	{:else if panel.s.error}
		<div class="file-preview-state file-preview-error">{panel.s.error}</div>
	{:else if panel.s.previewKind === 'image'}
		<div class="file-preview-image-wrap">
			<img src={panel.s.imageDataUrl} alt={filePath} class="file-preview-image" />
		</div>
	{:else if panel.s.previewKind === 'pdf'}
		<PdfPreview
			{workspacePath}
			{filePath}
			wiki={panel.isWikiFile}
			reloadVersion={panel.s.pdfReloadVersion}
		/>
	{:else if panel.s.previewKind === 'text'}
		<div class="file-preview-editor-wrap">
			{#if panel.s.externalComparisonOpen && panel.s.externalComparisonLines !== null}
				<FilePreviewExternal {panel} mode="diff" />
			{:else}
				{#if panel.showMarkdownToggle}
					<div class="md-view-toggle" role="group" aria-label="Markdown view mode">
						<button
							type="button"
							class="md-view-toggle-btn"
							class:active={panel.effectiveViewMode === 'preview'}
							onclick={() => panel.setViewMode('preview')}
						>
							Preview
						</button>
						<button
							type="button"
							class="md-view-toggle-btn"
							class:active={panel.effectiveViewMode === 'source'}
							onclick={() => panel.setViewMode('source')}
						>
							Source
						</button>
					</div>
				{/if}
				{#if panel.s.saveError}
					<div class="file-preview-save-error">{panel.s.saveError}</div>
				{/if}
				<FilePreviewExternal {panel} mode="notice" />
				{#if panel.effectiveViewMode === 'preview'}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div
						bind:this={panel.s.markdownScrollEl}
						class="file-preview-markdown scrollbar-none"
						onmouseup={panel.onPreviewMouseUp}
					>
						<AssistantMarkdown
							source={panel.s.draftContent}
							mode="assistant"
							annotateSourceLines
							wikiFiles={panel.s.wikiFiles}
							workspaceResources={{
								kind: panel.isWikiFile ? 'wiki' : 'workspace',
								workspacePath,
								filePath: panel.isWikiFile ? toWikiRelative(filePath) : filePath,
								readFile: (relativePath) =>
									panel.isWikiFile
										? readWikiFileContent(relativePath)
										: readWorkspaceFileContent(workspacePath, relativePath)
							}}
						/>
						{#if panel.showBacklinks}
							<FilePreviewBacklinks {panel} />
						{/if}
					</div>
				{:else}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class="file-preview-source-wrap" onmouseup={panel.onSourceMouseUp}>
						<FileEditor
							bind:this={panel.s.fileEditor}
							value={panel.s.draftContent}
							language={panel.s.language}
							readOnly={panel.s.saving || panel.readOnly}
							{revealRange}
							onChange={(value) => {
								panel.s.draftContent = value;
								if (panel.s.saveError) panel.s.saveError = null;
							}}
							onSave={() => {
								void panel.save();
							}}
							onRevealApplied={() => shellStore.clearFileRevealForActive()}
						/>
						{#if panel.showBacklinks && !panel.showMarkdownToggle}
							<FilePreviewBacklinks {panel} inSource />
						{/if}
					</div>
				{/if}
			{/if}
		</div>
	{/if}

	{#if panel.s.selectionPopup}
		<SelectionAddToChat
			position={{ top: panel.s.selectionPopup.top, left: panel.s.selectionPopup.left }}
			onAdd={panel.addSelectionToChat}
			onDismiss={panel.clearSelectionPopup}
		/>
	{/if}
</div>

<style>
	.file-preview {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: auto;
		background: var(--panel-bg);
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
		background: var(--panel-bg);
		color: var(--text-primary, var(--color-111111));
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
