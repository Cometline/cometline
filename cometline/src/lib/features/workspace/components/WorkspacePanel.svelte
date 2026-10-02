<script lang="ts">
	import { tick, untrack } from 'svelte';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import WorkspacePanelSurfaces from '#lib/features/workspace/components/workspace-panel/WorkspacePanelSurfaces.svelte';
	import WorkspacePanelToolbar from '#lib/features/workspace/components/workspace-panel/WorkspacePanelToolbar.svelte';
	import WorkspacePanelWebSearch from '#lib/features/workspace/components/workspace-panel/WorkspacePanelWebSearch.svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { isWorkspaceOwnedPane } from '#lib/features/workspace/workspace-pane-focus.js';
	import { createWorkspacePanelController } from '#lib/features/workspace/workspace-panel-controller.svelte.js';
	import { createWorkspacePanelView } from '#lib/features/workspace/workspace-panel-view.svelte.js';

	const view = createWorkspacePanelView();
	const panel = createWorkspacePanelController(view);

	/** Used by AppShell shared ⌘[ / ⌘] routing when the workspace panel is focused. */
	export function navigateBack() {
		panel.onBack();
	}

	export function navigateForward() {
		panel.onForward();
	}

	$effect(() => shellStore.registerPageContextResolver(panel.resolvePageContext));
	$effect(() => shellStore.registerWorkspacePanelLeaveGuard(panel.requestLeaveEditor));

	// Reads visibility only. noteVisibleContext does not expose the context list,
	// so this effect is not both reader and writer of the same state.
	$effect(() => panel.noteVisibleContext());

	$effect(() => {
		void view.panelSessionKey;
		panel.syncFilterFromStore();
	});

	$effect(() => {
		// Re-run whenever a focus is requested or the panel opens. The input may
		// already be mounted (panel was visible) so handle it here too; if it is
		// still mounting, trackAddressInput will pick up the same request id.
		const requestId = shellStore.addressBarFocusRequestId;
		const open = view.panelOpen;
		if (!requestId || !open) return;
		void tick().then(() =>
			requestAnimationFrame(() => requestAnimationFrame(() => panel.applyAddressFocus(true)))
		);
	});

	$effect(() => {
		const requestId = shellStore.fileTreeFilterFocusRequestId;
		const open = view.panelOpen;
		const filterVisible = view.showBrowseFilter;
		if (!requestId || !open || !filterVisible) return;
		void tick().then(() =>
			requestAnimationFrame(() =>
				requestAnimationFrame(() => panel.applyFileTreeFilterFocus(true))
			)
		);
	});

	$effect(() => {
		void view.webSurface;
		void view.panelFilePath;
		void view.panelFileTabs.length;
		void view.panelUrlTabs.length;
		void view.activeWebTabKey;
		void view.showWebview;
		void view.showFilePreview;
		if (!view.panelOpen) return;
		if (!isWorkspaceOwnedPane(untrack(() => shellStore.focusedPane))) return;
		void tick().then(() => panel.applyOwnedFocus());
	});
</script>

<svelte:window onkeydown={panel.handlePanelKeydown} />

<div class="workspace-panel" class:open={view.panelOpen} aria-hidden={!view.panelOpen}>
	<div
		bind:this={panel.panelFocusEl}
		class="workspace-panel-inner content-panel-surface"
		class:pane-focus-active={(shellStore.focusedPane === 'web' ||
			shellStore.focusedPane === 'terminal') &&
			view.panelOpen}
		tabindex="-1"
	>
		<WorkspacePanelToolbar {view} {panel} />
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="workspace-panel-content" onmousedown={panel.handlePanelMouseDown}>
			<WorkspacePanelSurfaces {view} {panel} />
			{#if view.showWebSearchField}
				<WorkspacePanelWebSearch {view} {panel} />
			{/if}
		</div>
	</div>
</div>

<ConfirmActionModal
	open={panel.terminateConfirmOpen}
	title="Terminate terminal?"
	description="This will stop the shell and every program started from it, including tmux, development servers, and SSH connections. This cannot be undone."
	confirmLabel="Terminate terminal"
	onCancel={() => (panel.terminateConfirmOpen = false)}
	onConfirm={() => void panel.confirmTerminateTerminal()}
/>

<ConfirmActionModal
	open={panel.discardChangesConfirmOpen}
	title="Discard unsaved changes?"
	description="Your edits to this file will be lost."
	confirmLabel="Discard changes"
	onCancel={() => panel.resolveLeaveEditor(false)}
	onConfirm={() => panel.resolveLeaveEditor(true)}
/>

<style>
	.workspace-panel {
		flex: 0 0 auto;
		width: 0;
		min-width: 0;
		height: 100%;
		overflow: hidden;
		pointer-events: none;
		box-sizing: border-box;
		transition: width var(--duration-fast) var(--ease-smooth);
	}

	.workspace-panel.open {
		width: var(--workspace-panel-slot-width);
		max-width: 100%;
		min-width: 0;
		flex-shrink: 1;
		pointer-events: auto;
	}

	.workspace-panel-inner {
		width: var(--workspace-panel-width);
		height: calc(100% - (2 * var(--content-panel-inset)));
		display: flex;
		flex-direction: column;
		margin: var(--content-panel-inset);
		margin-left: 0;
		box-sizing: border-box;
		overflow: hidden;
		transition:
			width var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.workspace-panel-inner:focus {
		outline: none;
	}

	.workspace-panel-content {
		flex: 1;
		min-height: 0;
		position: relative;
		background: var(--panel-bg);
		overflow: hidden;
	}

	@media (prefers-reduced-motion: reduce) {
		.workspace-panel {
			transition: none;
		}

		.workspace-panel-inner {
			transition: none;
		}
	}

	@media (max-width: 900px) {
		.workspace-panel {
			position: fixed;
			inset: 0;
			z-index: 40;
			width: 100% !important;
			transition: none;
			pointer-events: none;
		}

		.workspace-panel.open {
			pointer-events: auto;
		}

		.workspace-panel-inner {
			width: 100%;
			height: 100%;
			margin: 0;
			border: none;
			border-radius: 0;
			box-shadow: none;
			transform: translateX(100%);
			transition: transform var(--duration-fast) var(--ease-smooth);
		}

		.workspace-panel.open .workspace-panel-inner {
			transform: translateX(0);
		}
	}
</style>
