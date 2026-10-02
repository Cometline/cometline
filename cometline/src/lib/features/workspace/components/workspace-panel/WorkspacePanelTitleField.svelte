<script lang="ts">
	import PanelTabStrip from '#lib/features/workspace/components/PanelTabStrip.svelte';
	import { webTabActivity } from '#lib/features/workspace/web-tab-activity.svelte.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { isBlankTabUrl } from '#lib/features/workspace/workspace-panel-state.js';
	import type { WorkspacePanelController } from '#lib/features/workspace/workspace-panel-controller.svelte.js';
	import type { WorkspacePanelView } from '#lib/features/workspace/workspace-panel-view.svelte.js';

	let { view, panel }: { view: WorkspacePanelView; panel: WorkspacePanelController } = $props();
</script>

<div class="url-field">
	{#if view.showTerminalTitle}
		<span class="page-title">{view.surfaceTitle}</span>
	{:else if view.showFilePreview && view.panelFilePath}
		<PanelTabStrip
			tabs={view.panelFileTabs}
			activeId={view.panelFilePath}
			ariaLabel="Open files"
			dirtyById={panel.dirtyByPath}
			labelFor={(id) => id.split(/[/\\]/).pop() || id}
			onActivate={(id) => {
				shellStore.activateFileTabForActive(id);
				panel.applyOwnedFocus();
			}}
			onClose={(id) => void panel.closeFileTab(id)}
		/>
	{:else if view.showGitDiff && view.panelGitDiffPath}
		<span class="page-title">Diff</span>
		<span class="file-path-display" title={view.panelGitDiffPath}>{view.panelGitDiffPath}</span>
	{:else if view.showChangesTitle}
		<span class="page-title">{view.surfaceTitle}</span>
	{:else if view.showBrowseFilter}
		<div class="url-field-row">
			<span class="page-title surface-title">{view.surfaceTitle}</span>
			<input
				use:panel.trackFileTreeFilterInput
				class="address-input browse-filter-input"
				type="text"
				spellcheck="false"
				autocomplete="off"
				placeholder={view.webSurface === 'wiki'
					? 'Filter wiki files…'
					: 'Filter workspace files…'}
				value={panel.activeBrowseFilter}
				oninput={(event) =>
					panel.setActiveBrowseFilter((event.currentTarget as HTMLInputElement).value)}
				onfocus={panel.onFilterFocus}
				onkeydown={panel.onFilterKeydown}
				aria-label="Filter files"
			/>
		</div>
	{:else if view.showWebSearchField}
		<PanelTabStrip
			tabs={view.panelUrlTabs}
			activeId={view.panelUrlTabId}
			ariaLabel="Open pages"
			webStatusFor={(id) =>
				webTabActivity.get(`${view.panelSessionKey}:${id}`)?.surface.pageState}
			onToggleMute={(id) =>
				webTabActivity.get(`${view.panelSessionKey}:${id}`)?.surface.toggleAudioMuted()}
			labelFor={view.urlTabLabel}
			titleFor={(id) => {
				const url = view.panelUrlTabMeta[id]?.url ?? id;
				return isBlankTabUrl(url) ? 'New Tab' : url;
			}}
			onActivate={(id) => {
				shellStore.activateUrlTabForActive(id);
				panel.applyOwnedFocus();
			}}
			onClose={panel.closeUrlTab}
			onNewTab={panel.openNewWebTab}
		/>
	{:else}
		<span class="page-title">{view.surfaceTitle}</span>
	{/if}
</div>

<style>
	.url-field {
		position: relative;
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 1px;
		padding: 0 4px;
	}

	.url-field-row {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
	}

	.page-title {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.surface-title {
		flex-shrink: 0;
		max-width: 7.5rem;
	}

	.address-input {
		flex: 1;
		width: 100%;
		min-width: 0;
		border: none;
		background: transparent;
		font-size: 11px;
		color: var(--text-muted);
		padding: 0;
		outline: none;
	}

	.browse-filter-input {
		font-size: 12px;
		color: var(--text-main);
	}

	.address-input:focus {
		color: var(--text-main);
	}

	.address-input::placeholder {
		color: var(--text-muted);
		opacity: 0.7;
	}

	.file-path-display {
		width: 100%;
		min-width: 0;
		font-size: 11px;
		color: var(--text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
