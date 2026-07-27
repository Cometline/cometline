<script lang="ts">
	import { Copy, Loader, MessageSquarePlus, Minus, Plus, RotateCcw } from '@lucide/svelte';
	import {
		discardWorkspaceGitPaths,
		getWorkspaceGitDiff,
		getWorkspaceGitStatus,
		stageWorkspaceGitPaths,
		unstageWorkspaceGitPaths,
		type GitScope
	} from '$lib/client/cometmind';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import { portal } from '$lib/components/portal';
	import { shellStore } from '$lib/stores/shell.svelte';
	import {
		highlightGitDiffLines,
		type HighlightedDiffLine
	} from '$lib/workspace/git-diff-highlight';
	import { parseGitDiffLines } from '$lib/workspace/git-diff-lines';
	import {
		canStageGitFile,
		canUnstageGitFile,
		type GitFileStageState
	} from '$lib/workspace/git-file-state';
	import { languageFromPath } from '$lib/workspace/file-preview';
	import { workspaceChangeVersion } from '$lib/workspace/workspace-change.svelte';
	import { buildFileSnippetContext } from '$lib/workspace/selection-snippet';
	import {
		firstSelectionClientRect,
		selectionPopupPosition
	} from '$lib/workspace/selection-popup';

	let {
		workspacePath,
		filePath,
		scope = 'working' as GitScope,
		onBack,
		onMutated
	}: {
		workspacePath: string;
		filePath: string;
		scope?: GitScope;
		onBack?: () => void;
		onMutated?: () => void;
	} = $props();

	let loading = $state(true);
	let error = $state<string | null>(null);
	let diffText = $state('');
	let binary = $state(false);
	let empty = $state(false);
	let truncated = $state(false);
	let message = $state('');
	let loadSeq = 0;
	let copyFlash = $state(false);
	let mutating = $state(false);
	let actionError = $state<string | null>(null);
	let discardConfirmOpen = $state(false);
	let fileState = $state<GitFileStageState | null>(null);
	let highlightedLines = $state<HighlightedDiffLine[]>([]);
	let selectionPopup = $state<{
		top: number;
		left: number;
		text: string;
	} | null>(null);

	const fileName = $derived(filePath.split(/[/\\]/).filter(Boolean).pop() || filePath);
	const language = $derived(languageFromPath(filePath));
	const canStage = $derived(canStageGitFile(fileState));
	const canUnstage = $derived(canUnstageGitFile(fileState));
	const canDiscard = $derived(canStage);

	async function load() {
		const seq = ++loadSeq;
		loading = true;
		error = null;
		diffText = '';
		binary = false;
		empty = false;
		truncated = false;
		message = '';
		fileState = null;
		highlightedLines = [];
		selectionPopup = null;
		try {
			const [result, status] = await Promise.all([
				getWorkspaceGitDiff(workspacePath, filePath, scope),
				getWorkspaceGitStatus(workspacePath, 'all').catch(() => null)
			]);
			if (seq !== loadSeq) return;
			binary = Boolean(result.binary);
			empty = Boolean(result.empty);
			truncated = Boolean(result.truncated);
			message = result.message ?? '';
			diffText = result.diff ?? '';
			const match = status?.files?.find((f) => f.path === filePath);
			fileState = match
				? { staged: match.staged, untracked: match.untracked, xy: match.xy }
				: null;
			if (diffText.trim()) {
				const parsed = parseGitDiffLines(diffText);
				highlightedLines = await highlightGitDiffLines(parsed, languageFromPath(filePath));
			}
		} catch (err) {
			if (seq !== loadSeq) return;
			error = err instanceof Error ? err.message : 'Failed to load diff';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	async function copyPath() {
		try {
			await navigator.clipboard.writeText(filePath);
			copyFlash = true;
			setTimeout(() => {
				copyFlash = false;
			}, 1200);
		} catch {
			// ignore clipboard failures
		}
	}

	function addPathToChat() {
		shellStore.addWebContextForActive({
			kind: 'file',
			title: fileName,
			source: `workspace-file:${filePath}`,
			content: ''
		});
		shellStore.requestComposerFocus();
	}

	function addFullDiffToChat() {
		const content = diffText.trim().slice(0, 50000);
		if (!content) return;
		shellStore.addWebContextForActive({
			kind: 'file',
			title: `${fileName} (diff)`,
			source: `workspace-file:${filePath}`,
			content
		});
		shellStore.requestComposerFocus();
	}

	async function runMutation(action: () => Promise<unknown>) {
		if (mutating) return;
		mutating = true;
		actionError = null;
		try {
			await action();
			onMutated?.();
			await load();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Git action failed';
		} finally {
			mutating = false;
		}
	}

	function stageFile() {
		if (!canStage) return;
		return runMutation(() => stageWorkspaceGitPaths(workspacePath, [filePath]));
	}

	function unstageFile() {
		if (!canUnstage) return;
		return runMutation(() => unstageWorkspaceGitPaths(workspacePath, [filePath]));
	}

	function requestDiscard() {
		if (!canDiscard || mutating) return;
		discardConfirmOpen = true;
	}

	function confirmDiscard() {
		discardConfirmOpen = false;
		return runMutation(async () => {
			await discardWorkspaceGitPaths(workspacePath, [filePath]);
			onBack?.();
		});
	}

	function clearSelectionPopup() {
		selectionPopup = null;
	}

	function onDiffMouseUp(event: MouseEvent) {
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
		const text = sel.toString().trim();
		if (!text) {
			clearSelectionPopup();
			return;
		}
		const rect = firstSelectionClientRect(sel.getRangeAt(0));
		selectionPopup = {
			...selectionPopupPosition(rect, window.innerWidth),
			text
		};
	}

	function addSelectionToChat() {
		if (!selectionPopup) return;
		const ctx = buildFileSnippetContext({
			filePath,
			selectedText: selectionPopup.text
		});
		if (ctx) {
			// Prefer a diff-flavored title when the selection looks like a hunk.
			const title = selectionPopup.text.includes('\n')
				? `${fileName} (diff selection)`
				: ctx.title;
			shellStore.addWebContextForActive({
				...ctx,
				title
			});
			shellStore.requestComposerFocus();
		}
		clearSelectionPopup();
		window.getSelection()?.removeAllRanges();
	}

	$effect(() => {
		void [workspacePath, filePath, scope, workspaceChangeVersion(workspacePath)];
		void load();
	});

	$effect(() => {
		document.addEventListener('scroll', clearSelectionPopup, true);
		window.addEventListener('resize', clearSelectionPopup);
		return () => {
			document.removeEventListener('scroll', clearSelectionPopup, true);
			window.removeEventListener('resize', clearSelectionPopup);
		};
	});
</script>

<div class="git-diff-view">
	<header class="git-diff-header">
		<div class="git-diff-title-row">
			{#if onBack}
				<button type="button" class="git-diff-back" onclick={onBack}>← Files</button>
			{/if}
			<span class="git-diff-path" title={filePath}>{filePath}</span>
		</div>
		<div class="git-diff-actions">
			<button
				type="button"
				class="git-diff-action"
				disabled={mutating || !canStage}
				onclick={() => void stageFile()}
				title={canStage ? 'Stage' : 'Already staged'}
			>
				<Plus size={14} />
				<span>Stage</span>
			</button>
			<button
				type="button"
				class="git-diff-action"
				disabled={mutating || !canUnstage}
				onclick={() => void unstageFile()}
				title={canUnstage ? 'Unstage' : 'Nothing staged'}
			>
				<Minus size={14} />
				<span>Unstage</span>
			</button>
			<button
				type="button"
				class="git-diff-action danger"
				disabled={mutating || !canDiscard}
				onclick={requestDiscard}
				title={canDiscard ? 'Discard' : 'No working-tree changes to discard'}
			>
				<RotateCcw size={14} />
				<span>Discard</span>
			</button>
			<button
				type="button"
				class="git-diff-action"
				onclick={() => void copyPath()}
				title="Copy path"
			>
				<Copy size={14} />
				<span>{copyFlash ? 'Copied' : 'Copy path'}</span>
			</button>
			<button
				type="button"
				class="git-diff-action"
				onclick={addPathToChat}
				title="Add path to chat"
			>
				<MessageSquarePlus size={14} />
				<span>Add path</span>
			</button>
			{#if diffText.trim()}
				<button
					type="button"
					class="git-diff-action"
					onclick={addFullDiffToChat}
					title="Add full file diff to chat"
				>
					<span>Add diff</span>
				</button>
			{/if}
		</div>
		{#if actionError}
			<p class="git-diff-action-error" role="alert">{actionError}</p>
		{/if}
	</header>

	{#if loading}
		<div class="git-diff-state">
			<Loader size={16} stroke-width={2} class="git-diff-spinner" />
			<span>Loading diff…</span>
		</div>
	{:else if error}
		<div class="git-diff-state git-diff-error">{error}</div>
	{:else if binary}
		<div class="git-diff-state">{message || 'Binary file; diff not shown.'}</div>
	{:else if empty}
		<div class="git-diff-state">
			{message || 'No diff for this path in the selected scope.'}
		</div>
	{:else if !diffText.trim()}
		<div class="git-diff-state">{message || 'No diff available.'}</div>
	{:else}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="git-diff-body-wrap" onmouseup={onDiffMouseUp}>
			<!--
			  Keep zero whitespace between .diff-line nodes: this is a <pre>, so
			  newlines in the template would render as blank rows between every line.
			-->
			<pre class="git-diff-body scrollbar-none" data-lang={language ?? ''}><code
					><!-- prettier-ignore -->{#each highlightedLines as line, i (i)}<span class="diff-line kind-{line.kind}">{#if line.prefix}<span class="diff-prefix">{line.prefix}</span>{/if}<span class="diff-code">{@html line.html}</span></span>{/each}</code
				></pre>
			{#if truncated}
				<p class="git-diff-truncated">{message || 'Diff truncated.'}</p>
			{/if}
		</div>
	{/if}

	{#if selectionPopup}
		<button
			use:portal
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

<ConfirmActionModal
	open={discardConfirmOpen}
	title="Discard local changes?"
	description={`Discard changes to “${filePath}”? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`}
	confirmLabel="Discard"
	onCancel={() => (discardConfirmOpen = false)}
	onConfirm={() => void confirmDiscard()}
/>

<style>
	.git-diff-view {
		position: relative;
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: #fff;
	}

	.git-diff-header {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px 12px;
		border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
	}

	.git-diff-title-row {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.git-diff-back {
		flex: 0 0 auto;
		border: none;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		padding: 2px 0;
	}

	.git-diff-back:hover {
		color: var(--text-primary, #111);
	}

	.git-diff-path {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		font-weight: 600;
		color: var(--text-primary, #111);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	.git-diff-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.git-diff-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		border: 1px solid var(--border-soft, rgba(0, 0, 0, 0.1));
		border-radius: 7px;
		padding: 4px 8px;
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
		font-size: 12px;
		font-weight: 550;
		cursor: pointer;
	}

	.git-diff-action:hover:not(:disabled) {
		border-color: var(--text-soft, rgba(0, 0, 0, 0.2));
	}

	.git-diff-action:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.git-diff-action.danger:hover:not(:disabled) {
		border-color: rgba(185, 28, 28, 0.35);
		color: #b91c1c;
	}

	.git-diff-action-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error, #b91c1c);
	}

	.git-diff-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 24px;
		color: var(--text-muted);
		font-size: 13px;
		text-align: center;
	}

	.git-diff-error {
		color: var(--status-error, #b91c1c);
	}

	.git-diff-state :global(.git-diff-spinner) {
		animation: git-diff-spin 0.7s linear infinite;
	}

	@keyframes git-diff-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.git-diff-body-wrap {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.git-diff-body {
		flex: 1;
		min-height: 0;
		margin: 0;
		padding: 8px 0 16px;
		overflow: auto;
		font-size: 11.5px;
		line-height: 1.45;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		white-space: pre;
		user-select: text;
	}

	.diff-line {
		display: block;
		padding: 0 12px;
		white-space: pre;
	}

	.diff-prefix {
		display: inline;
	}

	.diff-code {
		display: inline;
	}

	.kind-meta,
	.kind-hunk,
	.kind-other {
		color: var(--text-muted);
	}

	.kind-add {
		background: rgba(34, 197, 94, 0.14);
	}

	.kind-add .diff-prefix {
		color: #15803d;
	}

	.kind-del {
		background: rgba(239, 68, 68, 0.14);
	}

	.kind-del .diff-prefix {
		color: #b91c1c;
	}

	.kind-ctx {
		color: var(--text-muted);
	}

	.kind-ctx .diff-prefix {
		color: var(--text-muted);
	}

	:global([data-theme='dark']) .kind-add,
	:global(.dark) .kind-add {
		background: rgba(34, 197, 94, 0.18);
	}

	:global([data-theme='dark']) .kind-add .diff-prefix,
	:global(.dark) .kind-add .diff-prefix {
		color: #86efac;
	}

	:global([data-theme='dark']) .kind-del,
	:global(.dark) .kind-del {
		background: rgba(239, 68, 68, 0.18);
	}

	:global([data-theme='dark']) .kind-del .diff-prefix,
	:global(.dark) .kind-del .diff-prefix {
		color: #fca5a5;
	}

	.git-diff-truncated {
		margin: 0;
		padding: 8px 12px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.selection-add-chat {
		position: fixed;
		z-index: 10000;
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
</style>
