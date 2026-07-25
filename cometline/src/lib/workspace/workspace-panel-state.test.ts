import { describe, expect, it } from 'vitest';
import {
	clearFileReveal,
	closeWorkspacePanel,
	createWorkspacePanelState,
	openWorkspacePanelFile,
	replacesActiveFile
} from './workspace-panel-state';

describe('workspace panel state', () => {
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

	it('identifies only file-replacing navigation as destructive', () => {
		const state = openWorkspacePanelFile(
			createWorkspacePanelState('workspace'),
			'workspace',
			'src/app.ts'
		);

		expect(replacesActiveFile(state, 'workspace', { mode: 'file', filePath: 'src/app.ts' })).toBe(
			false
		);
		expect(replacesActiveFile(state, 'wiki', { mode: 'file', filePath: '@runtime/wiki/index.md' })).toBe(
			true
		);
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
