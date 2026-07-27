import { describe, expect, it } from 'vitest';
import {
	applyWorkspaceChange,
	refreshWorkspace,
	workspaceChangeVersion,
	workspaceFileChangeVersion
} from './workspace-change.svelte';

describe('workspace change store', () => {
	it('tracks changed paths without reloading unrelated file previews', () => {
		const workspace = `/tmp/workspace-change-${crypto.randomUUID()}`;

		applyWorkspaceChange({
			workspacePath: workspace,
			paths: ['src\\app.ts'],
			gitChanged: false
		});

		expect(workspaceChangeVersion(workspace)).toBe(1);
		expect(workspaceFileChangeVersion(workspace, 'src/app.ts')).toBe(1);
		expect(workspaceFileChangeVersion(workspace, 'README.md')).toBe(0);
	});

	it('refreshes all file previews for an unknown external change', () => {
		const workspace = `/tmp/workspace-refresh-${crypto.randomUUID()}`;

		refreshWorkspace(workspace);

		expect(workspaceChangeVersion(workspace)).toBe(1);
		expect(workspaceFileChangeVersion(workspace, 'README.md')).toBe(1);
	});

	it('keeps Git-only changes out of file preview refreshes', () => {
		const workspace = `/tmp/workspace-git-${crypto.randomUUID()}`;

		applyWorkspaceChange({ workspacePath: workspace, paths: [], gitChanged: true });

		expect(workspaceChangeVersion(workspace)).toBe(1);
		expect(workspaceFileChangeVersion(workspace, 'README.md')).toBe(0);
	});
});
