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
		wikiTabs = [],
		workspaceTabs = [],
		wikiFilePath,
		workspaceFilePath,
		wikiRevealRange = null,
		workspaceRevealRange = null,
		activeSurface,
		active,
		onEditorState,
		onDirtyByPath
	}: {
		workspacePath: string;
		wikiTabs?: string[];
		workspaceTabs?: string[];
		wikiFilePath: string | null;
		workspaceFilePath: string | null;
		wikiRevealRange?: FileRevealRange | null;
		workspaceRevealRange?: FileRevealRange | null;
		activeSurface: 'wiki' | 'workspace' | 'changes' | 'web-search';
		active: boolean;
		onEditorState: (state: FileEditorState | null) => void;
		onDirtyByPath?: (dirtyByPath: Record<string, boolean>) => void;
	} = $props();

	const wikiPaths = $derived(
		wikiTabs.length > 0 ? wikiTabs : wikiFilePath ? [wikiFilePath] : []
	);
	const workspacePaths = $derived(
		workspaceTabs.length > 0 ? workspaceTabs : workspaceFilePath ? [workspaceFilePath] : []
	);

	let wikiEditorStateByPath = $state<Record<string, FileEditorState | null>>({});
	let workspaceEditorStateByPath = $state<Record<string, FileEditorState | null>>({});
	let lastDirtyPayload = '';

	function editorStateEquals(a: FileEditorState | null, b: FileEditorState | null): boolean {
		if (a === b) return true;
		if (!a || !b) return false;
		return (
			a.dirty === b.dirty &&
			a.saving === b.saving &&
			a.saveError === b.saveError &&
			a.save === b.save &&
			a.revert === b.revert
		);
	}

	function setWikiEditorState(path: string, state: FileEditorState | null) {
		if (editorStateEquals(wikiEditorStateByPath[path] ?? null, state)) return;
		wikiEditorStateByPath = { ...wikiEditorStateByPath, [path]: state };
	}

	function setWorkspaceEditorState(path: string, state: FileEditorState | null) {
		if (editorStateEquals(workspaceEditorStateByPath[path] ?? null, state)) return;
		workspaceEditorStateByPath = { ...workspaceEditorStateByPath, [path]: state };
	}

	const activeEditorState = $derived(
		active && activeSurface === 'wiki' && wikiFilePath
			? (wikiEditorStateByPath[wikiFilePath] ?? null)
			: active && activeSurface === 'workspace' && workspaceFilePath
				? (workspaceEditorStateByPath[workspaceFilePath] ?? null)
				: null
	);

	const dirtyByPath = $derived.by(() => {
		const next: Record<string, boolean> = {};
		for (const path of wikiPaths) {
			next[path] = Boolean(wikiEditorStateByPath[path]?.dirty);
		}
		for (const path of workspacePaths) {
			next[path] = Boolean(workspaceEditorStateByPath[path]?.dirty);
		}
		return next;
	});

	$effect(() => onEditorState(activeEditorState));
	$effect(() => {
		const payload = JSON.stringify(dirtyByPath);
		if (payload === lastDirtyPayload) return;
		lastDirtyPayload = payload;
		onDirtyByPath?.(dirtyByPath);
	});
</script>

<div class="file-surfaces">
	{#each wikiPaths as path (path)}
		<div
			class="panel-layer panel-layer-content"
			class:active={active && activeSurface === 'wiki' && wikiFilePath === path}
			inert={!(active && activeSurface === 'wiki' && wikiFilePath === path)}
			aria-hidden={!(active && activeSurface === 'wiki' && wikiFilePath === path)}
		>
			<FilePreview
				{workspacePath}
				filePath={path}
				revealRange={wikiFilePath === path ? wikiRevealRange : null}
				onEditorState={(state) => setWikiEditorState(path, state)}
			/>
		</div>
	{/each}
	{#each workspacePaths as path (path)}
		<div
			class="panel-layer panel-layer-content"
			class:active={active && activeSurface === 'workspace' && workspaceFilePath === path}
			inert={!(active && activeSurface === 'workspace' && workspaceFilePath === path)}
			aria-hidden={!(active && activeSurface === 'workspace' && workspaceFilePath === path)}
		>
			<FilePreview
				{workspacePath}
				filePath={path}
				revealRange={workspaceFilePath === path ? workspaceRevealRange : null}
				onEditorState={(state) => setWorkspaceEditorState(path, state)}
			/>
		</div>
	{/each}
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
		pointer-events: none;
		visibility: hidden;
		z-index: 1;
	}

	.panel-layer.active {
		pointer-events: auto;
		visibility: visible;
		z-index: 3;
	}
</style>
