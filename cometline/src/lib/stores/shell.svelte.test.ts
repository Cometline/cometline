import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const getActiveSessionId = vi.hoisted(() => vi.fn<() => string | null>(() => null));

vi.mock('$lib/active-session', () => ({
	getActiveSessionId
}));

import { shellStore } from './shell.svelte';

describe('shellStore default vs active workspace', () => {
	beforeEach(() => {
		getActiveSessionId.mockReturnValue(null);
		shellStore.initializeDefaultWorkspace('/default');
	});

	it('initializeDefaultWorkspace sets default and active in sync', () => {
		expect(shellStore.defaultWorkspacePath).toBe('/default');
		expect(shellStore.workspacePath).toBe('/default');
		expect(shellStore.sidebarOrderWorkspacePath).toBe('/default');
	});

	it('setDefaultWorkspacePath syncs active when no session is open', () => {
		shellStore.commitActiveWorkspace('/temporary');

		shellStore.setDefaultWorkspacePath('/new-default');

		expect(shellStore.defaultWorkspacePath).toBe('/new-default');
		expect(shellStore.workspacePath).toBe('/new-default');
		expect(shellStore.sidebarOrderWorkspacePath).toBe('/new-default');
	});

	it('setDefaultWorkspacePath leaves active unchanged when a session is open', () => {
		shellStore.commitActiveWorkspace('/session-ws');
		getActiveSessionId.mockReturnValue('sess-1');

		shellStore.setDefaultWorkspacePath('/new-default');

		expect(shellStore.defaultWorkspacePath).toBe('/new-default');
		expect(shellStore.workspacePath).toBe('/session-ws');
		expect(shellStore.sidebarOrderWorkspacePath).toBe('/session-ws');
	});

	it('commitActiveWorkspace updates active and sidebar order without touching default', () => {
		shellStore.commitActiveWorkspace('/fork-target');

		expect(shellStore.defaultWorkspacePath).toBe('/default');
		expect(shellStore.workspacePath).toBe('/fork-target');
		expect(shellStore.sidebarOrderWorkspacePath).toBe('/fork-target');
		expect(shellStore.sidebarOrderDiscordActive).toBe(false);
	});

	it('resetActiveToDefault restores active and sidebar order to default', () => {
		shellStore.commitActiveWorkspace('/fork-target');

		shellStore.resetActiveToDefault();

		expect(shellStore.defaultWorkspacePath).toBe('/default');
		expect(shellStore.workspacePath).toBe('/default');
		expect(shellStore.sidebarOrderWorkspacePath).toBe('/default');
	});
});

describe('shellStore web panel focus behavior', () => {
	beforeEach(() => {
		vi.stubGlobal('window', { electronAPI: undefined });
		getActiveSessionId.mockReturnValue('sess-1');
		shellStore.clearWebPanelForSession('sess-1');
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('no-ops panel opens when no session is active', () => {
		getActiveSessionId.mockReturnValue(null);
		const before = shellStore.addressBarFocusRequestId;
		shellStore.openWebPanelForActive('https://example.com');
		shellStore.openFilePreviewForActive('README.md');
		expect(shellStore.webPanelOpen).toBe(false);
		expect(shellStore.addressBarFocusRequestId).toBe(before);
		expect(shellStore.webPanelSessionKey).toBeNull();
	});

	it('does not focus the address bar when opening a URL from app content', () => {
		const before = shellStore.addressBarFocusRequestId;

		shellStore.openWebPanelForActive('https://example.com');

		expect(shellStore.addressBarFocusRequestId).toBe(before);
		shellStore.clearWebPanelForSession('sess-1');
	});

	it('requests file-tree filter focus when opening an empty web panel', () => {
		const addressBefore = shellStore.addressBarFocusRequestId;
		const filterBefore = shellStore.fileTreeFilterFocusRequestId;

		shellStore.openWebPanelEmpty();

		expect(shellStore.fileTreeFilterFocusRequestId).toBe(filterBefore + 1);
		expect(shellStore.addressBarFocusRequestId).toBe(addressBefore);
		shellStore.clearWebPanelForSession('sess-1');
	});

	it('cycles ⌘O focus between filter and address while browsing', () => {
		shellStore.openWebPanelEmpty();
		const filterAfterOpen = shellStore.fileTreeFilterFocusRequestId;
		const addressAfterOpen = shellStore.addressBarFocusRequestId;

		shellStore.openWebPanelFromShortcut();
		expect(shellStore.addressBarFocusRequestId).toBe(addressAfterOpen + 1);
		expect(shellStore.fileTreeFilterFocusRequestId).toBe(filterAfterOpen);

		shellStore.openWebPanelFromShortcut();
		expect(shellStore.fileTreeFilterFocusRequestId).toBe(filterAfterOpen + 1);

		shellStore.clearWebPanelForSession('sess-1');
	});

	it('tracks browse → file history and restores browse on back', () => {
		shellStore.openWebPanelEmpty();
		expect(shellStore.webPanelBrowseOpen).toBe(true);

		shellStore.openFilePreviewForActive('@runtime/wiki/index.md');
		expect(shellStore.webPanelMode).toBe('file');
		expect(shellStore.webPanelFilePath).toBe('@runtime/wiki/index.md');
		expect(shellStore.canPanelHistoryBack).toBe(true);

		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelBrowseOpen).toBe(true);
		expect(shellStore.canPanelHistoryForward).toBe(true);

		expect(shellStore.panelHistoryForward()).toBe(true);
		expect(shellStore.webPanelFilePath).toBe('@runtime/wiki/index.md');

		shellStore.closeWebPanel();
		expect(shellStore.canPanelHistoryBack).toBe(false);
		expect(shellStore.webPanelOpen).toBe(false);
		shellStore.clearWebPanelForSession('sess-1');
	});

	it('seeds browse under a direct file open so back returns to the tree', () => {
		shellStore.openFilePreviewForActive('README.md');
		expect(shellStore.canPanelHistoryBack).toBe(true);

		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelBrowseOpen).toBe(true);

		shellStore.clearWebPanelForSession('sess-1');
	});

	it('keeps file-tree expansion when opening a nested file and going back', () => {
		shellStore.setWebPanelBrowseSource('workspace');
		shellStore.setFileTreeExpanded('workspace', { src: true, 'src/lib': true });

		shellStore.openFilePreviewForActive('src/lib/components/WebPanel.svelte');
		// Parents of the opened file are merged into expansion.
		expect(shellStore.getFileTreeExpanded('workspace')).toMatchObject({
			src: true,
			'src/lib': true,
			'src/lib/components': true
		});

		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelBrowseOpen).toBe(true);
		// Expansion still present after back (tree remount reads this map).
		expect(shellStore.getFileTreeExpanded('workspace')['src/lib/components']).toBe(true);

		shellStore.clearWebPanelForSession('sess-1');
		expect(shellStore.getFileTreeExpanded('workspace')).toEqual({});
	});

	it('expands wiki tree parents using the relative path under @runtime/wiki/', () => {
		shellStore.setWebPanelBrowseSource('wiki');
		shellStore.openFilePreviewForActive('@runtime/wiki/topics/setup.md');
		expect(shellStore.getFileTreeExpanded('wiki')).toEqual({ topics: true });
		shellStore.clearWebPanelForSession('sess-1');
	});

	it('records history across browse sources, files, and git diffs', () => {
		shellStore.setWebPanelBrowseSource('wiki');
		expect(shellStore.webPanelBrowseOpen).toBe(true);
		expect(shellStore.webPanelBrowseSource).toBe('wiki');

		shellStore.setWebPanelBrowseSource('workspace');
		expect(shellStore.webPanelBrowseSource).toBe('workspace');
		expect(shellStore.canPanelHistoryBack).toBe(true);

		shellStore.openFilePreviewForActive('src/app.ts');
		expect(shellStore.webPanelMode).toBe('file');

		shellStore.setWebPanelBrowseSource('changes');
		expect(shellStore.webPanelBrowseOpen).toBe(true);
		expect(shellStore.webPanelBrowseSource).toBe('changes');

		shellStore.openGitDiffForActive('src/app.ts');
		expect(shellStore.webPanelMode).toBe('git-diff');
		expect(shellStore.webPanelGitDiffPath).toBe('src/app.ts');

		// Back: changes browse
		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelBrowseOpen).toBe(true);
		expect(shellStore.webPanelBrowseSource).toBe('changes');

		// Back: file preview
		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelFilePath).toBe('src/app.ts');

		// Back: workspace browse
		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelBrowseSource).toBe('workspace');

		// Back: wiki browse
		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.webPanelBrowseSource).toBe('wiki');

		// Forward to workspace again
		expect(shellStore.panelHistoryForward()).toBe(true);
		expect(shellStore.webPanelBrowseSource).toBe('workspace');

		shellStore.clearWebPanelForSession('sess-1');
	});
});

describe('shellStore terminal panel visibility', () => {
	beforeEach(() => {
		vi.stubGlobal('window', { electronAPI: undefined });
		getActiveSessionId.mockReturnValue('sess-1');
		shellStore.clearWebPanelForSession('sess-1');
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('hides the terminal panel without changing its running state', () => {
		const focusBefore = shellStore.terminalFocusRequestId;
		expect(shellStore.openTerminalPanel()).toBe(true);
		expect(shellStore.terminalPanelOpen).toBe(true);
		expect(shellStore.terminalFocusRequestId).toBe(focusBefore + 1);

		shellStore.closeWorkspacePanel();

		expect(shellStore.terminalPanelOpen).toBe(false);
	});

	it('closes the active terminal panel when its shell exits', () => {
		shellStore.openTerminalPanel();

		shellStore.closeTerminalPanelForSession('sess-1');

		expect(shellStore.terminalPanelOpen).toBe(false);
		expect(shellStore.focusedPane).toBe('chat');
	});
});

describe('shellStore lazy page context', () => {
	beforeEach(() => {
		vi.stubGlobal('window', { electronAPI: undefined });
		getActiveSessionId.mockReturnValue('sess-1');
		shellStore.clearWebPanelForSession('sess-1');
		shellStore.clearWebContextForActive();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('keeps page navigation as metadata until a resolver is requested', async () => {
		const resolvePage = vi.fn(async (source: string) => ({
			kind: 'page' as const,
			title: 'Example',
			source,
			content: 'Captured only when sending'
		}));
		const unregister = shellStore.registerPageContextResolver(resolvePage);
		shellStore.setPendingPageContextForActive({
			title: 'Example',
			source: 'https://example.com'
		});

		expect(shellStore.pendingWebContexts).toEqual([
			{ kind: 'page', title: 'Example', source: 'https://example.com', lazy: true }
		]);
		expect(resolvePage).not.toHaveBeenCalled();

		await expect(shellStore.resolvePendingWebContextsForActive()).resolves.toEqual([
			{
				kind: 'page',
				title: 'Example',
				source: 'https://example.com',
				content: 'Captured only when sending'
			}
		]);
		expect(resolvePage).toHaveBeenCalledWith('https://example.com');
		unregister();
	});

	it('appends snippets and upserts viewing path without wiping others', async () => {
		shellStore.setViewingFileContextForActive('workspace-file:a.md', 'a.md');
		shellStore.addWebContextForActive({
			kind: 'file',
			title: 'a.md:1-2',
			source: 'workspace-file:a.md#L1-L2',
			content: 'hello'
		});
		shellStore.setPendingPageContextForActive({
			title: 'Example',
			source: 'https://example.com'
		});
		expect(shellStore.pendingWebContexts).toHaveLength(3);

		shellStore.setViewingFileContextForActive('workspace-file:b.md', 'b.md');
		expect(
			shellStore.pendingWebContexts.filter((c) => 'role' in c && c.role === 'viewing')
		).toEqual([
			{
				kind: 'file',
				role: 'viewing',
				title: 'b.md',
				source: 'workspace-file:b.md',
				content: ''
			}
		]);
		expect(shellStore.pendingWebContexts).toHaveLength(3);

		shellStore.removeWebContextAt(0); // drop the snippet; keep page + viewing
		expect(shellStore.pendingWebContexts).toHaveLength(2);

		const resolvePage = vi.fn(async (source: string) => ({
			kind: 'page' as const,
			title: 'Example',
			source,
			content: 'page body'
		}));
		const unregister = shellStore.registerPageContextResolver(resolvePage);
		await expect(shellStore.resolvePendingWebContextsForActive()).resolves.toEqual([
			{
				kind: 'page',
				title: 'Example',
				source: 'https://example.com',
				content: 'page body'
			},
			{
				kind: 'file',
				title: 'b.md',
				source: 'workspace-file:b.md',
				content: ''
			}
		]);
		unregister();
	});
});
