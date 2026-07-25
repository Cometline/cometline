<script lang="ts">
	import FilePreview from '$lib/components/FilePreview.svelte';
	import type { FileRevealRange } from '$lib/workspace/workspace-panel-state';

	type FileEditorState = {
		dirty: boolean;
		saving: boolean;
		saveError: string | null;
		save: () => Promise<void>;
		revert: () => void;
	};

	let {
		workspacePath,
		wikiFilePath,
		workspaceFilePath,
		wikiRevealRange = null,
		workspaceRevealRange = null,
		activeSurface,
		active,
		onEditorState
	}: {
		workspacePath: string;
		wikiFilePath: string | null;
		workspaceFilePath: string | null;
		wikiRevealRange?: FileRevealRange | null;
		workspaceRevealRange?: FileRevealRange | null;
		activeSurface: 'wiki' | 'workspace' | 'changes' | 'web-search';
		active: boolean;
		onEditorState: (state: FileEditorState | null) => void;
	} = $props();

	let wikiEditorState = $state<FileEditorState | null>(null);
	let workspaceEditorState = $state<FileEditorState | null>(null);
	const activeEditorState = $derived(
		active && activeSurface === 'wiki'
			? wikiEditorState
			: active && activeSurface === 'workspace'
				? workspaceEditorState
				: null
	);

	$effect(() => onEditorState(activeEditorState));
</script>

<div class="file-surfaces">
	{#if wikiFilePath}
		<div class="panel-layer panel-layer-content" class:active={active && activeSurface === 'wiki'}>
			<FilePreview
				{workspacePath}
				filePath={wikiFilePath}
				revealRange={wikiRevealRange}
				onEditorState={(state) => (wikiEditorState = state)}
			/>
		</div>
	{/if}
	{#if workspaceFilePath}
		<div class="panel-layer panel-layer-content" class:active={active && activeSurface === 'workspace'}>
			<FilePreview
				{workspacePath}
				filePath={workspaceFilePath}
				revealRange={workspaceRevealRange}
				onEditorState={(state) => (workspaceEditorState = state)}
			/>
		</div>
	{/if}
</div>

<style>
	.file-surfaces {
		display: contents;
	}

	.panel-layer {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: #fff;
		transform: translateX(100%);
		opacity: 0;
		pointer-events: none;
		visibility: hidden;
		transition:
			transform 180ms var(--ease-smooth, ease),
			opacity 180ms var(--ease-smooth, ease),
			visibility 180ms;
		z-index: 1;
	}

	.panel-layer.active {
		transform: translateX(0);
		opacity: 1;
		pointer-events: auto;
		visibility: visible;
		z-index: 3;
	}
</style>
