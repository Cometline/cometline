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

describe('shellStore workspace panel focus behavior', () => {
	beforeEach(() => {
		vi.stubGlobal('window', { electronAPI: undefined });
		getActiveSessionId.mockReturnValue('sess-1');
		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('no-ops panel opens when no session is active', () => {
		getActiveSessionId.mockReturnValue(null);
		const before = shellStore.addressBarFocusRequestId;
		shellStore.openWorkspacePanelUrlForActive('https://example.com');
		shellStore.openFilePreviewForActive('README.md');
		expect(shellStore.workspacePanelOpen).toBe(false);
		expect(shellStore.addressBarFocusRequestId).toBe(before);
		expect(shellStore.workspacePanelSessionKey).toBeNull();
	});

	it('does not focus the address bar when opening a URL from app content', () => {
		const before = shellStore.addressBarFocusRequestId;

		shellStore.openWorkspacePanelUrlForActive('https://example.com');

		expect(shellStore.addressBarFocusRequestId).toBe(before);
		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('requests file-tree filter focus when opening an empty workspace panel', () => {
		const addressBefore = shellStore.addressBarFocusRequestId;
		const filterBefore = shellStore.fileTreeFilterFocusRequestId;

		shellStore.openWorkspacePanelBrowse();

		expect(shellStore.fileTreeFilterFocusRequestId).toBe(filterBefore + 1);
		expect(shellStore.addressBarFocusRequestId).toBe(addressBefore);
		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('⌘O opens web search (address focus)', () => {
		shellStore.openWorkspacePanelBrowse();
		expect(shellStore.lastWorkspacePanelFocusTarget).toBe('filter');

		shellStore.openWebSearchPanel();
		expect(shellStore.lastWorkspacePanelFocusTarget).toBe('address');
		expect(shellStore.workspacePanelSurface).toBe('web');

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('openWebSearchPanel switches from terminal surface to web address', () => {
		shellStore.openWorkspacePanelBrowse();
		shellStore.requestTerminalFocus();
		expect(shellStore.workspacePanelSurface).toBe('terminal');

		shellStore.openWebSearchPanel();
		expect(shellStore.workspacePanelSurface).toBe('web');
		expect(shellStore.contentSurface).toBe('web-search');
		expect(shellStore.lastWorkspacePanelFocusTarget).toBe('address');

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('wiki/workspace shortcuts leave the terminal surface', () => {
		shellStore.openWorkspacePanelBrowse();
		shellStore.requestTerminalFocus();
		expect(shellStore.workspacePanelSurface).toBe('terminal');

		shellStore.setWorkspacePanelBrowseSource('wiki');
		expect(shellStore.workspacePanelSurface).toBe('web');
		expect(shellStore.workspacePanelBrowseSource).toBe('wiki');
		expect(shellStore.lastWorkspacePanelFocusTarget).toBe('filter');

		shellStore.requestTerminalFocus();
		shellStore.setWorkspacePanelBrowseSource('workspace');
		expect(shellStore.workspacePanelSurface).toBe('web');
		expect(shellStore.workspacePanelBrowseSource).toBe('workspace');

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('keeps independent content per surface and preserves per-source filters', () => {
		shellStore.setWorkspacePanelBrowseSource('wiki');
		shellStore.setFileTreeFilter('wiki', 'readme');
		shellStore.setFileTreeFilter('workspace', 'main');

		shellStore.openFilePreviewForActive('@runtime/wiki/index.md');
		expect(shellStore.contentSurface).toBe('wiki');
		expect(shellStore.workspacePanelMode).toBe('file');
		expect(shellStore.getSurfaceContent('wiki')).toEqual({
			mode: 'file',
			filePath: '@runtime/wiki/index.md'
		});

		shellStore.openFilePreviewForActive('src/app.ts');
		expect(shellStore.contentSurface).toBe('workspace');
		expect(shellStore.getSurfaceContent('workspace')).toEqual({
			mode: 'file',
			filePath: 'src/app.ts'
		});
		// Wiki file remains owned by the wiki surface.
		expect(shellStore.getSurfaceContent('wiki')).toEqual({
			mode: 'file',
			filePath: '@runtime/wiki/index.md'
		});

		shellStore.setWorkspacePanelBrowseSource('wiki');
		expect(shellStore.contentSurface).toBe('wiki');
		expect(shellStore.workspacePanelFilePath).toBe('@runtime/wiki/index.md');
		expect(shellStore.getFileTreeFilter('wiki')).toBe('readme');
		expect(shellStore.getFileTreeFilter('workspace')).toBe('main');

		shellStore.setWorkspacePanelBrowseSource('workspace');
		expect(shellStore.contentSurface).toBe('workspace');
		expect(shellStore.workspacePanelFilePath).toBe('src/app.ts');
		expect(shellStore.getFileTreeFilter('workspace')).toBe('main');

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('keeps the active file when its leave guard rejects navigation', async () => {
		await shellStore.openFilePreviewForActive('src/app.ts');
		const leaveGuard = vi.fn(async () => false);
		const unregister = shellStore.registerWorkspacePanelLeaveGuard(leaveGuard);

		await expect(shellStore.openFilePreviewForActive('src/main.ts')).resolves.toBe(false);
		expect(leaveGuard).toHaveBeenCalledOnce();
		expect(shellStore.workspacePanelFilePath).toBe('src/app.ts');
		expect(shellStore.getSurfaceContent('workspace')).toEqual({
			mode: 'file',
			filePath: 'src/app.ts'
		});

		unregister();
		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('tracks browse → file history and restores browse on back', () => {
		shellStore.openWorkspacePanelBrowse();
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);

		shellStore.openFilePreviewForActive('@runtime/wiki/index.md');
		expect(shellStore.workspacePanelMode).toBe('file');
		expect(shellStore.workspacePanelFilePath).toBe('@runtime/wiki/index.md');
		expect(shellStore.canPanelHistoryBack).toBe(true);

		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);
		expect(shellStore.canPanelHistoryForward).toBe(true);

		expect(shellStore.panelHistoryForward()).toBe(true);
		expect(shellStore.workspacePanelFilePath).toBe('@runtime/wiki/index.md');

		// Layered Cmd+W: first dismisses content, second soft-hides.
		shellStore.closeWorkspacePanel();
		expect(shellStore.workspacePanelOpen).toBe(true);
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);
		expect(shellStore.getSurfaceContent('wiki')).toBeNull();

		shellStore.closeWorkspacePanel();
		expect(shellStore.workspacePanelOpen).toBe(false);
		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('Cmd+W walks content dots one surface at a time before soft-hiding', () => {
		shellStore.openFilePreviewForActive('@runtime/wiki/index.md');
		shellStore.openFilePreviewForActive('src/app.ts');
		shellStore.openWorkspacePanelUrlForActive('https://example.com');
		expect(shellStore.contentSurface).toBe('web-search');
		expect(shellStore.workspacePanelUrl).toBe('https://example.com');
		expect(shellStore.getSurfaceContent('wiki')).not.toBeNull();
		expect(shellStore.getSurfaceContent('workspace')).not.toBeNull();

		// 1) Dismiss web page → stay on web search.
		shellStore.closeWorkspacePanel();
		expect(shellStore.workspacePanelOpen).toBe(true);
		expect(shellStore.contentSurface).toBe('web-search');
		expect(shellStore.getSurfaceContent('web-search')).toBeNull();
		expect(shellStore.getSurfaceContent('wiki')).not.toBeNull();
		expect(shellStore.getSurfaceContent('workspace')).not.toBeNull();

		// 2) Leave empty web search → jump to next content dot (wiki).
		shellStore.closeWorkspacePanel();
		expect(shellStore.workspacePanelOpen).toBe(true);
		expect(shellStore.contentSurface).toBe('wiki');
		expect(shellStore.workspacePanelFilePath).toBe('@runtime/wiki/index.md');

		// 3) Dismiss wiki file → wiki browse.
		shellStore.closeWorkspacePanel();
		expect(shellStore.contentSurface).toBe('wiki');
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);
		expect(shellStore.getSurfaceContent('wiki')).toBeNull();

		// 4) Leave wiki browse → jump to workspace content.
		shellStore.closeWorkspacePanel();
		expect(shellStore.contentSurface).toBe('workspace');
		expect(shellStore.workspacePanelFilePath).toBe('src/app.ts');

		// 5) Dismiss workspace file.
		shellStore.closeWorkspacePanel();
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);
		expect(shellStore.getSurfaceContent('workspace')).toBeNull();

		// 6) No remaining dots → soft-hide; content stays cleared but trees retained.
		shellStore.closeWorkspacePanel();
		expect(shellStore.workspacePanelOpen).toBe(false);
		expect(shellStore.getSurfaceContent('wiki')).toBeNull();
		expect(shellStore.getSurfaceContent('workspace')).toBeNull();

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('seeds browse under a direct file open so back returns to the tree', () => {
		shellStore.openFilePreviewForActive('README.md');
		expect(shellStore.canPanelHistoryBack).toBe(true);

		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('keeps file-tree expansion when opening a nested file and going back', () => {
		shellStore.setWorkspacePanelBrowseSource('workspace');
		shellStore.setFileTreeExpanded('workspace', { src: true, 'src/lib': true });

		shellStore.openFilePreviewForActive('src/lib/components/WorkspacePanel.svelte');
		// Parents of the opened file are merged into expansion.
		expect(shellStore.getFileTreeExpanded('workspace')).toMatchObject({
			src: true,
			'src/lib': true,
			'src/lib/components': true
		});

		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);
		// Expansion still present after back (tree remount reads this map).
		expect(shellStore.getFileTreeExpanded('workspace')['src/lib/components']).toBe(true);

		shellStore.clearWorkspacePanelForSession('sess-1');
		expect(shellStore.getFileTreeExpanded('workspace')).toEqual({});
	});

	it('expands wiki tree parents using the relative path under @runtime/wiki/', () => {
		shellStore.setWorkspacePanelBrowseSource('wiki');
		shellStore.openFilePreviewForActive('@runtime/wiki/topics/setup.md');
		expect(shellStore.getFileTreeExpanded('wiki')).toEqual({ topics: true });
		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('keeps per-surface history stacks independent', () => {
		shellStore.setWorkspacePanelBrowseSource('wiki');
		shellStore.openFilePreviewForActive('@runtime/wiki/index.md');
		expect(shellStore.workspacePanelFilePath).toBe('@runtime/wiki/index.md');

		shellStore.setWorkspacePanelBrowseSource('workspace');
		shellStore.openFilePreviewForActive('src/app.ts');
		expect(shellStore.workspacePanelFilePath).toBe('src/app.ts');

		// Back on workspace returns to workspace browse, not the wiki file.
		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.contentSurface).toBe('workspace');
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);
		expect(shellStore.getSurfaceContent('wiki')).toEqual({
			mode: 'file',
			filePath: '@runtime/wiki/index.md'
		});

		shellStore.setWorkspacePanelBrowseSource('changes');
		shellStore.openGitDiffForActive('src/app.ts');
		expect(shellStore.workspacePanelMode).toBe('git-diff');
		expect(shellStore.panelHistoryBack()).toBe(true);
		expect(shellStore.workspacePanelBrowseSource).toBe('changes');
		expect(shellStore.workspacePanelBrowseOpen).toBe(true);

		shellStore.clearWorkspacePanelForSession('sess-1');
	});

	it('soft-hides terminal on Cmd+W without clearing web surface content', () => {
		shellStore.openFilePreviewForActive('src/app.ts');
		expect(shellStore.openTerminalPanel()).toBe(true);
		expect(shellStore.workspacePanelSurface).toBe('terminal');

		shellStore.closeWorkspacePanel();
		expect(shellStore.terminalPanelOpen).toBe(false);
		expect(shellStore.getSurfaceContent('workspace')).toEqual({
			mode: 'file',
			filePath: 'src/app.ts'
		});

		shellStore.setWorkspacePanelBrowseSource('workspace');
		expect(shellStore.workspacePanelOpen).toBe(true);
		expect(shellStore.workspacePanelFilePath).toBe('src/app.ts');

		shellStore.clearWorkspacePanelForSession('sess-1');
	});
});

describe('shellStore terminal panel visibility', () => {
	beforeEach(() => {
		vi.stubGlobal('window', { electronAPI: undefined });
		getActiveSessionId.mockReturnValue('sess-1');
		shellStore.clearWorkspacePanelForSession('sess-1');
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

	it('requestTerminalFocus does not double-bump the focus request id', () => {
		const focusBefore = shellStore.terminalFocusRequestId;
		shellStore.requestTerminalFocus();
		expect(shellStore.terminalPanelOpen).toBe(true);
		expect(shellStore.terminalFocusRequestId).toBe(focusBefore + 1);
		shellStore.clearWorkspacePanelForSession('sess-1');
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
		shellStore.clearWorkspacePanelForSession('sess-1');
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

	it('deduplicates the same assistant selection but keeps distinct snippets', () => {
		shellStore.addWebContextForActive({
			kind: 'message',
			title: 'First selected response',
			source: 'assistant-response://sess-1/1',
			content: 'First selected\nresponse'
		});
		shellStore.addWebContextForActive({
			kind: 'message',
			title: 'First selected response',
			source: 'assistant-response://sess-1/1',
			content: ' First  selected response '
		});
		shellStore.addWebContextForActive({
			kind: 'message',
			title: 'Different selection',
			source: 'assistant-response://sess-1/1',
			content: 'Different selection'
		});

		expect(shellStore.pendingWebContexts).toHaveLength(2);
	});
});
