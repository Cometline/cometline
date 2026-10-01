<script lang="ts">
	import FileTreeBrowser from '$lib/features/workspace/components/FileTreeBrowser.svelte';
	import WorkspaceFileSurface from '$lib/features/workspace/components/WorkspaceFileSurface.svelte';
	import GitChangesBrowser from '$lib/features/workspace/components/GitChangesBrowser.svelte';
	import GitDiffView from '$lib/features/workspace/components/GitDiffView.svelte';
	import TerminalPanel from '$lib/features/workspace/components/TerminalPanel.svelte';
	import WorkspaceWebSurface from '$lib/features/workspace/components/WorkspaceWebSurface.svelte';
	import WorkspacePanelLayer from '$lib/features/workspace/components/workspace-panel/WorkspacePanelLayer.svelte';
	import { webTabActivity } from '$lib/features/workspace/web-tab-activity.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import type { WorkspacePanelController } from '$lib/features/workspace/workspace-panel-controller.svelte';
	import type { WorkspacePanelView } from '$lib/features/workspace/workspace-panel-view.svelte';

	let { view, panel }: { view: WorkspacePanelView; panel: WorkspacePanelController } = $props();
</script>

<WorkspacePanelLayer variant="terminal" active={view.terminalLayerActive}>
	<TerminalPanel bind:this={panel.terminalPanelRef} active={view.terminalLayerActive} />
</WorkspacePanelLayer>
{#if shellStore.hasWorkspacePanelForSession}
	<WorkspacePanelLayer active={view.wikiLayerActive}>
		<FileTreeBrowser
			bind:this={panel.wikiTreeBrowser}
			source="wiki"
			workspacePath={shellStore.workspacePath}
			filter={panel.wikiFilter}
			onSelectFile={(path) => void panel.openTreeFile(path)}
		/>
	</WorkspacePanelLayer>
	<WorkspacePanelLayer active={view.workspaceLayerActive}>
		<FileTreeBrowser
			bind:this={panel.workspaceTreeBrowser}
			source="workspace"
			workspacePath={shellStore.workspacePath}
			filter={panel.workspaceFilter}
			onSelectFile={(path) => void panel.openTreeFile(path)}
		/>
	</WorkspacePanelLayer>
	<WorkspacePanelLayer active={view.changesLayerActive}>
		<GitChangesBrowser workspacePath={view.normalizedWorkspacePath} />
	</WorkspacePanelLayer>
	<WorkspaceFileSurface
		workspacePath={shellStore.workspacePath}
		wikiTabs={view.wikiFileTabs}
		workspaceTabs={view.workspaceSurfaceFileTabs}
		wikiFilePath={view.wikiFilePath}
		workspaceFilePath={view.workspaceFilePath}
		wikiRevealRange={view.wikiRevealRange}
		workspaceRevealRange={view.workspaceRevealRange}
		activeSurface={view.webSurface}
		active={view.onWebSurface}
		onEditorState={(state) => (panel.editorState = state)}
		onDirtyByPath={panel.setDirtyByPath}
	/>
	{#if view.changesDiffPath}
		<WorkspacePanelLayer variant="content" active={view.changesDiffActive}>
			<GitDiffView
				workspacePath={shellStore.workspacePath}
				filePath={view.changesDiffPath}
				scope="all"
				onBack={() => shellStore.panelHistoryBack()}
			/>
		</WorkspacePanelLayer>
	{/if}
{/if}
{#each view.webTabs as tab (tab.key)}
	{@const active = view.panelOpen && view.showWebview && tab.key === view.activeWebTabKey}
	<WorkspacePanelLayer variant="content" {active}>
		<WorkspaceWebSurface
			bind:this={
				() => webTabActivity.get(tab.key)?.surface,
				(surface) => {
					if (surface)
						webTabActivity.set(tab.key, {
							sessionId: tab.sessionId,
							tabId: tab.id,
							surface
						});
					else webTabActivity.delete(tab.key);
				}
			}
			url={tab.url}
			sessionKey={tab.key}
			onNavigationState={(state) =>
				shellStore.syncWorkspacePanelUrlFromGuest(
					tab.sessionId,
					tab.id,
					state.url,
					state.title
				)}
			onFocus={() => {
				if (active) shellStore.setFocusedPane('web');
			}}
			onNewWindow={(url) => {
				if (active) panel.onNewWindow(url);
			}}
		/>
	</WorkspacePanelLayer>
{/each}
