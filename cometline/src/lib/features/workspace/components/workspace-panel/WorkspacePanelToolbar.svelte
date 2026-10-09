<script lang="ts">
	import {
		ArrowLeft,
		ArrowRight,
		BookOpen,
		FolderTree,
		GitBranch,
		Play,
		Power,
		RotateCcw,
		Save,
		Search,
		SquareTerminal
	} from '@lucide/svelte';
	import { tick } from 'svelte';
	import Tooltip from '#lib/components/Tooltip.svelte';
	import WorkspacePanelTitleField from '#lib/features/workspace/components/workspace-panel/WorkspacePanelTitleField.svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import type { WorkspacePanelController } from '#lib/features/workspace/workspace-panel-controller.svelte.js';
	import type { WorkspacePanelView } from '#lib/features/workspace/workspace-panel-view.svelte.js';

	let { view, panel }: { view: WorkspacePanelView; panel: WorkspacePanelController } = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<header class="workspace-panel-toolbar" onmousedown={panel.handlePanelMouseDown}>
	<div class="nav-actions">
		<Tooltip
			label={view.terminalAvailable ? 'Terminal' : 'Start a chat to use Terminal'}
			action={view.terminalAvailable ? 'openTerminal' : undefined}
		>
			<button
				type="button"
				class="icon-button"
				class:active={view.onTerminalSurface}
				disabled={!view.terminalAvailable}
				onmousedown={panel.keepPaneFocus}
				onclick={() => shellStore.requestTerminalFocus()}
				aria-label={view.terminalAvailable ? 'Terminal' : 'Start a chat to use Terminal'}
			>
				<SquareTerminal size={16} />
			</button>
		</Tooltip>
		<Tooltip label="Wiki files" action="openWikiPanel">
			<button
				type="button"
				class="icon-button"
				class:active={view.wikiActive}
				onmousedown={panel.keepPaneFocus}
				onclick={() => panel.switchBrowseSource('wiki')}
				aria-label="Wiki files"
			>
				<BookOpen size={16} />
				{#if view.wikiHasContentDot}
					<span class="surface-content-dot" aria-hidden="true"></span>
				{/if}
			</button>
		</Tooltip>
		<Tooltip
			label={view.workspaceAvailable
				? 'Workspace files'
				: 'Select a workspace to browse files'}
			action={view.workspaceAvailable ? 'openWorkspacePanel' : undefined}
		>
			<button
				type="button"
				class="icon-button"
				class:active={view.workspaceActive}
				disabled={!view.workspaceAvailable}
				onmousedown={panel.keepPaneFocus}
				onclick={() => panel.switchBrowseSource('workspace')}
				aria-label={view.workspaceAvailable
					? 'Workspace files'
					: 'Select a workspace to browse files'}
			>
				<FolderTree size={16} />
				{#if view.workspaceHasContentDot}
					<span class="surface-content-dot" aria-hidden="true"></span>
				{/if}
			</button>
		</Tooltip>
		<Tooltip label="Web search" action="openWebSearch">
			<button
				type="button"
				class="icon-button"
				class:active={view.webSearchActive}
				onmousedown={panel.keepPaneFocus}
				onclick={() => {
					shellStore.openWebSearchPanel();
					void tick().then(() => panel.applyOwnedFocus());
				}}
				aria-label="Web search"
			>
				<Search size={16} />
				{#if view.webSearchHasContentDot}
					<span class="surface-content-dot" aria-hidden="true"></span>
				{/if}
			</button>
		</Tooltip>
		<Tooltip
			label={view.workspaceAvailable
				? 'Git changes'
				: 'Select a workspace to see git changes'}
			action={view.workspaceAvailable ? 'openGitPanel' : undefined}
		>
			<button
				type="button"
				class="icon-button"
				class:active={view.changesActive}
				disabled={!view.workspaceAvailable}
				onmousedown={panel.keepPaneFocus}
				onclick={() => shellStore.openGitChangesPanel()}
				aria-label={view.workspaceAvailable
					? 'Git changes'
					: 'Select a workspace to see git changes'}
			>
				<GitBranch size={16} />
				{#if view.changesHasContentDot}
					<span class="surface-content-dot" aria-hidden="true"></span>
				{/if}
			</button>
		</Tooltip>
		<Tooltip label="Back" action="navigateBack">
			<button
				type="button"
				class="icon-button"
				disabled={!view.toolbarCanGoBack}
				onclick={panel.onBack}
				aria-label="Back"
			>
				<ArrowLeft size={16} />
			</button>
		</Tooltip>
		<Tooltip label="Forward" action="navigateForward">
			<button
				type="button"
				class="icon-button"
				disabled={!view.toolbarCanGoForward}
				onclick={panel.onForward}
				aria-label="Forward"
			>
				<ArrowRight size={16} />
			</button>
		</Tooltip>
	</div>
	<WorkspacePanelTitleField {view} {panel} />
	{#if view.showTerminalTitle}
		<div class="file-actions">
			{#if view.activeTerminal?.status === 'running'}
				<button
					type="button"
					class="icon-button"
					onclick={() => (panel.terminateConfirmOpen = true)}
					aria-label="Terminate terminal"
					title="Terminate terminal"
				>
					<Power size={16} />
				</button>
			{:else}
				<button
					type="button"
					class="icon-button"
					onclick={() => void panel.terminalPanelRef?.startTerminal()}
					disabled={!view.terminalAvailable}
					aria-label="Start terminal"
					title="Start terminal"
				>
					<Play size={16} />
				</button>
			{/if}
		</div>
	{:else if view.showFilePreview && panel.editorState}
		<div class="file-actions">
			<button
				type="button"
				class="icon-button"
				disabled={!panel.dirty || panel.saving}
				onclick={panel.onRevertClick}
				aria-label="Revert changes"
				title="Revert changes"
			>
				<RotateCcw size={16} />
			</button>
			<button
				type="button"
				class="icon-button"
				disabled={!panel.dirty || panel.saving}
				onclick={panel.onSaveClick}
				aria-label="Save file"
				title="Save (Cmd/Ctrl+S)"
			>
				<Save size={16} />
			</button>
		</div>
	{/if}
</header>

<style>
	.workspace-panel-toolbar {
		display: flex;
		align-items: center;
		gap: 8px;
		box-sizing: border-box;
		height: var(--panel-header-height);
		padding: 0 10px;
		overflow: hidden;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 250, 249, 0.95);
	}

	.nav-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
	}

	.file-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
	}

	.icon-button {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-main);
		cursor: pointer;
	}

	.icon-button:hover:not(:disabled) {
		background: rgba(15, 23, 42, 0.06);
	}

	.icon-button.active {
		background: rgba(15, 23, 42, 0.08);
	}

	.icon-button:disabled {
		opacity: 0.35;
		cursor: default;
	}

	.surface-content-dot {
		position: absolute;
		top: 3px;
		right: 3px;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: var(--hero-composer-glow-color, var(--color-72c0ff));
		box-shadow: 0 0 8px var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24));
		pointer-events: none;
	}

	:global(.spin) {
		animation: workspace-panel-spin 0.8s linear infinite;
	}

	@keyframes workspace-panel-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
