import { describe, expect, it } from 'vitest';
import {
	activateWorkspacePanelFileTab,
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

		expect(replacesActiveFile(state, 'workspace', { mode: 'file', filePath: 'src/app.ts' })).toBe(
			false
		);
		expect(
			replacesActiveFile(state, 'workspace', { mode: 'file', filePath: 'src/other.ts' })
		).toBe(false);
		expect(replacesActiveFile(state, 'wiki', { mode: 'file', filePath: '@runtime/wiki/index.md' })).toBe(
			false
		);
	});

	it('adds and activates url tabs; Cmd+W closes active url tab first', () => {
		let state = openWorkspacePanelUrl(createWorkspacePanelState('web-search'), 'https://a.example');
		state = openWorkspacePanelUrl(state, 'https://b.example');
		expect(urlTabsFor(state)).toEqual(['https://a.example', 'https://b.example']);
		expect(state.content['web-search']).toEqual({ mode: 'url', url: 'https://b.example' });

		state = closeWorkspacePanel(state);
		expect(urlTabsFor(state)).toEqual(['https://a.example']);
		expect(state.content['web-search']).toEqual({ mode: 'url', url: 'https://a.example' });

		state = openWorkspacePanelUrl(state, 'https://b.example');
		state = closeWorkspacePanelUrlTab(state, 'https://a.example');
		expect(urlTabsFor(state)).toEqual(['https://b.example']);
		expect(state.content['web-search']).toEqual({ mode: 'url', url: 'https://b.example' });
	});

	it('address-bar navigate replaces the active url tab', () => {
		let state = openWorkspacePanelUrl(createWorkspacePanelState('web-search'), 'https://a.example');
		state = openWorkspacePanelUrl(state, 'https://b.example');
		state = navigateWorkspacePanelUrl(state, 'https://c.example');
		expect(urlTabsFor(state)).toEqual(['https://a.example', 'https://c.example']);
		expect(state.content['web-search']).toEqual({ mode: 'url', url: 'https://c.example' });
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
