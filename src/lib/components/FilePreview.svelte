<script lang="ts">
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import { Loader } from '@lucide/svelte';
	import { readWorkspaceFileContent } from '$lib/client/cometmind';
	import { getHighlighter, resolveLanguage, CODE_THEME } from '$lib/markdown/highlight';
	import { isMarkdownPath, languageFromExtension, languageFromPath } from '$lib/workspace/file-preview';

	let {
		workspacePath,
		filePath
	}: {
		workspacePath: string;
		filePath: string;
	} = $props();

	let loading = $state(true);
	let error = $state<string | null>(null);
	let textContent = $state('');
	let imageDataUrl = $state('');
	let codeHtml = $state('');
	let previewKind = $state<'markdown' | 'code' | 'text' | 'image' | null>(null);
	let loadVersion = 0;

	async function loadPreview() {
		const version = ++loadVersion;
		loading = true;
		error = null;
		textContent = '';
		imageDataUrl = '';
		codeHtml = '';
		previewKind = null;

		try {
			const result = await readWorkspaceFileContent(workspacePath, filePath);
			if (version !== loadVersion) return;

			if (result.kind === 'image') {
				previewKind = 'image';
				imageDataUrl = result.data_url;
				return;
			}

			textContent = result.content;
			if (isMarkdownPath(filePath)) {
				previewKind = 'markdown';
				return;
			}

			const language = languageFromPath(filePath) ?? languageFromExtension(result.extension);
			if (language) {
				try {
					const highlighter = await getHighlighter();
					if (version !== loadVersion) return;
					const resolved = resolveLanguage(highlighter, language);
					if (resolved) {
						codeHtml = highlighter.codeToHtml(textContent, {
							lang: resolved,
							theme: CODE_THEME
						});
						previewKind = 'code';
						return;
					}
				} catch {
					// Fall through to plain text.
				}
			}

			previewKind = 'text';
		} catch (err) {
			if (version !== loadVersion) return;
			error = err instanceof Error ? err.message : 'Failed to load file';
		} finally {
			if (version === loadVersion) loading = false;
		}
	}

	$effect(() => {
		workspacePath;
		filePath;
		void loadPreview();
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
	{:else if previewKind === 'markdown'}
		<div class="file-preview-markdown">
			<AssistantMarkdown source={textContent} />
		</div>
	{:else if previewKind === 'code' && codeHtml}
		<div class="file-preview-code">
			<!-- eslint-disable-next-line svelte/no-at-html-tags -->
			{@html codeHtml}
		</div>
	{:else if previewKind === 'text'}
		<pre class="file-preview-plain">{textContent}</pre>
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

	.file-preview-markdown {
		padding: 16px 18px 24px;
	}

	.file-preview-code {
		padding: 12px;
	}

	.file-preview-code :global(pre) {
		margin: 0;
		padding: 12px 14px;
		border-radius: 10px;
		overflow: auto;
		font-size: 12px;
		line-height: 1.5;
	}

	.file-preview-plain {
		margin: 0;
		padding: 16px 18px;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		color: var(--text-main);
	}

	@media (prefers-reduced-motion: reduce) {
		.file-preview-state :global(.file-preview-spinner) {
			animation: none;
		}
	}
</style>
