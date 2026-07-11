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
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('does not focus the address bar when opening a URL from app content', () => {
		const before = shellStore.addressBarFocusRequestId;

		shellStore.openWebPanelForActive('https://example.com');

		expect(shellStore.addressBarFocusRequestId).toBe(before);
		shellStore.closeWebPanel();
	});

	it('requests address focus when opening an empty web panel', () => {
		const before = shellStore.addressBarFocusRequestId;

		shellStore.openWebPanelEmpty();

		expect(shellStore.addressBarFocusRequestId).toBe(before + 1);
		shellStore.closeWebPanel();
	});
});

describe('shellStore lazy page context', () => {
	beforeEach(() => {
		getActiveSessionId.mockReturnValue(null);
		shellStore.clearWebContextForActive();
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
});
