<script lang="ts">
	import { Loader } from '@lucide/svelte';
	import FileEditor from '$lib/components/FileEditor.svelte';
	import {
		readWorkspaceFileContent,
		writeWorkspaceFileContent
	} from '$lib/client/cometmind';
	import { languageFromExtension, languageFromPath } from '$lib/workspace/file-preview';

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
		onEditorState
	}: {
		workspacePath: string;
		filePath: string;
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

	const dirty = $derived(previewKind === 'text' && draftContent !== savedContent);

	function revert() {
		if (previewKind !== 'text') return;
		draftContent = savedContent;
		saveError = null;
	}

	async function save() {
		if (previewKind !== 'text' || saving || !dirty) return;

		const nextContent = draftContent;
		const currentWorkspacePath = workspacePath;
		const currentFilePath = filePath;

		saving = true;
		saveError = null;
		try {
			await writeWorkspaceFileContent(currentWorkspacePath, currentFilePath, nextContent);
			if (workspacePath !== currentWorkspacePath || filePath !== currentFilePath) return;
			savedContent = nextContent;
			draftContent = nextContent;
		} catch (err) {
			if (workspacePath !== currentWorkspacePath || filePath !== currentFilePath) return;
			saveError = err instanceof Error ? err.message : 'Failed to save file';
		} finally {
			if (workspacePath === currentWorkspacePath && filePath === currentFilePath) {
				saving = false;
			}
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

		try {
			const result = await readWorkspaceFileContent(workspacePath, filePath);
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
		onEditorState?.(
			previewKind === 'text' && !loading && !error
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

<div class="file-preview" aria-live="polite">
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
			{#if saveError}
				<div class="file-preview-save-error">{saveError}</div>
			{/if}
			<FileEditor
				value={draftContent}
				language={language}
				readOnly={saving}
				onChange={(value) => {
					draftContent = value;
					if (saveError) saveError = null;
				}}
				onSave={() => {
					void save();
				}}
			/>
		</div>
	{/if}
</div>

<style>
	.file-preview {
		width: 100%;
		height: 100%;
		overflow: auto;
		background: #fff;
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
		color: #b42318;
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

	.file-preview-save-error {
		padding: 10px 14px;
		border-bottom: 1px solid rgba(180, 35, 24, 0.15);
		background: rgba(180, 35, 24, 0.05);
		color: #b42318;
		font-size: 12px;
	}

	@media (prefers-reduced-motion: reduce) {
		.file-preview-state :global(.file-preview-spinner) {
			animation: none;
		}
	}
</style>
