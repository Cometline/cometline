import { describe, expect, it } from 'vitest';
import {
	activateWorkspacePanelFileTab,
	activateWorkspacePanelUrlTab,
	clearFileReveal,
	closeWorkspacePanel,
	closeWorkspacePanelFileTab,
	closeWorkspacePanelUrlTab,
	createWorkspacePanelState,
	fileTabsFor,
	navigateWorkspacePanelUrl,
	openWorkspacePanelFile,
	openWorkspacePanelUrl,
	replacesActiveFile,
	syncActiveUrlTab,
	urlTabChipLabel,
	urlTabMetaFor,
	urlTabsFor
} from './workspace-panel-state';

describe('workspace panel state', () => {
	it('adds and activates file tabs instead of replacing', () => {
		let state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/a.ts'
		);
		state = openWorkspacePanelFile(state, 'workspace', 'src/b.ts');

		expect(fileTabsFor(state, 'workspace')).toEqual(['src/a.ts', 'src/b.ts']);
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/b.ts' });

		state = activateWorkspacePanelFileTab(state, 'workspace', 'src/a.ts');
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/a.ts' });
		expect(fileTabsFor(state, 'workspace')).toEqual(['src/a.ts', 'src/b.ts']);
	});

	it('closes the active tab before clearing the surface', () => {
		let state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/a.ts'
		);
		state = openWorkspacePanelFile(state, 'workspace', 'src/b.ts');
		state = openWorkspacePanelFile(state, 'workspace', 'src/c.ts');
		// active = c
		state = closeWorkspacePanel(state);
		expect(fileTabsFor(state, 'workspace')).toEqual(['src/a.ts', 'src/b.ts']);
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/b.ts' });

		state = activateWorkspacePanelFileTab(state, 'workspace', 'src/a.ts');
		state = closeWorkspacePanel(state);
		expect(fileTabsFor(state, 'workspace')).toEqual(['src/b.ts']);
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/b.ts' });

		state = closeWorkspacePanel(state);
		expect(state.content.workspace).toBeUndefined();
		expect(fileTabsFor(state, 'workspace')).toEqual([]);
	});

	it('closes an inactive tab without changing the active file', () => {
		let state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/a.ts'
		);
		state = openWorkspacePanelFile(state, 'workspace', 'src/b.ts');
		state = openWorkspacePanelFile(state, 'workspace', 'src/c.ts');
		// active = c; close inactive a
		state = closeWorkspacePanelFileTab(state, 'workspace', 'src/a.ts');
		expect(fileTabsFor(state, 'workspace')).toEqual(['src/b.ts', 'src/c.ts']);
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/c.ts' });
	});

	it('walks remaining content before hiding the panel', () => {
		let state = openWorkspacePanelFile(
			createWorkspacePanelState('wiki'),
			'wiki',
			'@runtime/wiki/index.md'
		);
		state = openWorkspacePanelFile(state, 'workspace', 'src/app.ts');

		state = closeWorkspacePanel(state);
		expect(state.content.workspace).toBeUndefined();

		state = closeWorkspacePanel(state);
		expect(state.contentSurface).toBe('wiki');
		expect(state.content.wiki).toEqual({ mode: 'file', filePath: '@runtime/wiki/index.md' });

		state = closeWorkspacePanel(state);
		state = closeWorkspacePanel(state);
		expect(state.visible).toBe(false);
	});

	it('soft-hides terminal without clearing web content', () => {
		const state = closeWorkspacePanel({
			...openWorkspacePanelFile(createWorkspacePanelState('workspace'), 'workspace', 'src/app.ts'),
			surface: 'terminal',
			terminalVisible: true
		});

		expect(state.terminalVisible).toBe(false);
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/app.ts' });
	});

	it('does not treat file tab open/activate as destructive', () => {
		const state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/app.ts'
		);

		expect(replacesActiveFile(state, { mode: 'file', filePath: 'src/app.ts' })).toBe(false);
		expect(replacesActiveFile(state, { mode: 'file', filePath: 'src/other.ts' })).toBe(false);
		expect(
			replacesActiveFile(state, { mode: 'file', filePath: '@runtime/wiki/index.md' })
		).toBe(false);
	});

	it('treats non-file navigation as destructive while a file is active', () => {
		const state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/app.ts'
		);

		expect(replacesActiveFile(state, { mode: 'url', url: 'https://example.com' })).toBe(true);
		expect(replacesActiveFile(state, { mode: 'git-diff', filePath: 'src/app.ts' })).toBe(true);
		expect(replacesActiveFile(state, null)).toBe(true);
	});

	it('adds and activates url tabs; Cmd+W closes active url tab first', () => {
		let state = openWorkspacePanelUrl(createWorkspacePanelState('web-search'), 'https://a.example');
		const [tabA] = urlTabsFor(state);
		state = openWorkspacePanelUrl(state, 'https://b.example');
		const [stillA, tabB] = urlTabsFor(state);
		expect(stillA).toBe(tabA);
		expect(tabB).not.toBe(tabA);
		expect(state.content['web-search']).toMatchObject({
			mode: 'url',
			url: 'https://b.example',
			tabId: tabB
		});

		state = closeWorkspacePanel(state);
		expect(urlTabsFor(state)).toEqual([tabA]);
		expect(state.content['web-search']).toMatchObject({
			mode: 'url',
			url: 'https://a.example',
			tabId: tabA
		});

		state = openWorkspacePanelUrl(state, 'https://b.example');
		const [, reopenedB] = urlTabsFor(state);
		state = closeWorkspacePanelUrlTab(state, tabA);
		expect(urlTabsFor(state)).toEqual([reopenedB]);
		expect(state.content['web-search']).toMatchObject({
			mode: 'url',
			url: 'https://b.example',
			tabId: reopenedB
		});
	});

	it('guest navigation keeps the tab id and remembers the page title', () => {
		let state = openWorkspacePanelUrl(
			createWorkspacePanelState('web-search'),
			'https://google.com/search?q=youtube'
		);
		const [searchTab] = urlTabsFor(state);
		state = openWorkspacePanelUrl(state, 'https://example.com');
		const [, exampleTab] = urlTabsFor(state);
		state = activateWorkspacePanelUrlTab(state, searchTab);
		state = syncActiveUrlTab(state, 'https://www.youtube.com/', 'YouTube');
		expect(urlTabsFor(state)).toEqual([searchTab, exampleTab]);
		expect(state.content['web-search']).toMatchObject({
			mode: 'url',
			url: 'https://www.youtube.com/',
			tabId: searchTab,
			title: 'YouTube'
		});
		expect(urlTabMetaFor(state, searchTab)).toEqual({
			url: 'https://www.youtube.com/',
			title: 'YouTube'
		});
		expect(urlTabChipLabel(urlTabMetaFor(state, searchTab))).toBe('YouTube');

		state = syncActiveUrlTab(state, 'https://www.youtube.com/watch?v=1');
		expect(urlTabsFor(state)).toEqual([searchTab, exampleTab]);
		expect(urlTabChipLabel(urlTabMetaFor(state, searchTab))).toBe('YouTube');
	});

	it('address-bar navigate keeps the tab id and clears the remembered title', () => {
		let state = openWorkspacePanelUrl(createWorkspacePanelState('web-search'), 'https://a.example');
		const [tabA] = urlTabsFor(state);
		state = openWorkspacePanelUrl(state, 'https://b.example');
		const [stillA, tabB] = urlTabsFor(state);
		expect(stillA).toBe(tabA);
		state = navigateWorkspacePanelUrl(state, 'https://c.example');
		expect(urlTabsFor(state)).toEqual([tabA, tabB]);
		expect(state.content['web-search']).toMatchObject({
			mode: 'url',
			url: 'https://c.example',
			tabId: tabB,
			title: ''
		});
	});

	it('stores and clears one-shot file reveal ranges', () => {
		let state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/app.ts',
			{ startLine: 2, endLine: 4 }
		);
		expect(state.content.workspace).toEqual({
			mode: 'file',
			filePath: 'src/app.ts',
			startLine: 2,
			endLine: 4
		});

		state = clearFileReveal(state, 'workspace');
		expect(state.content.workspace).toEqual({ mode: 'file', filePath: 'src/app.ts' });
	});
});
