// @vitest-environment jsdom
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { createRawSnippet, flushSync } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

// Keep the real shell and session stores, but exclude unrelated child lifecycles.
const { emptyComponent } = vi.hoisted(() => ({ emptyComponent: () => ({}) }));
vi.mock('./Sidebar.svelte', () => ({ default: emptyComponent }));
vi.mock('./FileTreeBrowser.svelte', () => ({ default: emptyComponent }));
vi.mock('./WorkspaceFileSurface.svelte', () => ({ default: emptyComponent }));
vi.mock('./WorkspaceWebSurface.svelte', () => ({ default: emptyComponent }));
vi.mock('./GitChangesBrowser.svelte', () => ({ default: emptyComponent }));
vi.mock('./GitDiffView.svelte', () => ({ default: emptyComponent }));
vi.mock('./TerminalPanel.svelte', () => ({ default: emptyComponent }));
vi.mock('./RuntimeOverlay.svelte', () => ({ default: emptyComponent }));
vi.mock('./SettingsModal.svelte', () => ({ default: emptyComponent }));
vi.mock('./onboarding/SetupWizard.svelte', () => ({ default: emptyComponent }));
vi.mock('./UpdateButton.svelte', () => ({ default: emptyComponent }));
vi.mock('./MemoryToast.svelte', () => ({ default: emptyComponent }));
vi.mock('./AppToast.svelte', () => ({ default: emptyComponent }));
vi.mock('./ConfirmActionModal.svelte', () => ({ default: emptyComponent }));
vi.mock('./FileSearchModal.svelte', () => ({ default: emptyComponent }));
vi.mock('./inbox/InboxDrawer.svelte', () => ({ default: emptyComponent }));
vi.mock('$app/state', () => ({ page: { url: new URL('http://localhost/session/focus-test') } }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import AppShell from './AppShell.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

const session: Session = {
	id: 'focus-test',
	workspace_id: 'workspace',
	workspace_path: '/repo',
	title: 'Focus test',
	model_id: 'model',
	provider_id: 'provider',
	status: 'active',
	origin: 'user',
	agent_mode: 'auto',
	pinned: false,
	running: false,
	token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
	created_at: 0,
	updated_at: 0
};

describe('AppShell pane focus', () => {
	beforeEach(() => {
		vi.stubGlobal(
			'ResizeObserver',
			class {
				observe() {}
				disconnect() {}
			}
		);
		shellStore.closeIntro();
		shellStore.clearWorkspacePanelForSession(session.id);
		shellStore.clearWorkspacePanelForSession('other-session');
		sessionStore.selectSession(session);
	});

	afterEach(() => {
		cleanup();
		shellStore.clearWorkspacePanelForSession(session.id);
		shellStore.clearWorkspacePanelForSession('other-session');
		sessionStore.selectSession(null);
		vi.unstubAllGlobals();
	});

	it.each(['@runtime/wiki/first.md', 'src/first.ts'])(
		'keeps the workspace ring after opening %s and updating Viewing context',
		async (path) => {
			const { container } = render(AppShell, {
				children: createRawSnippet(() => ({ render: () => '<textarea></textarea>' }))
			});
			flushSync();
			const focusRequest = shellStore.composerFocusRequest.id;

			await shellStore.openFilePreviewForActive(path);
			shellStore.setViewingFileContextForActive(`workspace-file:${path}`, path);
			flushSync();

			expect(shellStore.focusedPane).toBe('web');
			expect(container.querySelector('main')).not.toHaveClass('pane-focus-active');
			expect(shellStore.composerFocusRequest.id).toBe(focusRequest);
		}
	);

	it.each(['file', 'url'] as const)(
		'keeps the workspace ring after closing inactive and active %s tabs',
		async (kind) => {
			const paths = ['a', 'b', 'c'];
			for (const path of paths) {
				if (kind === 'file') await shellStore.openFilePreviewForActive(`${path}.md`);
				else shellStore.openWorkspacePanelUrlForActive(`https://${path}.example`);
			}
			const { container } = render(AppShell, {
				children: createRawSnippet(() => ({ render: () => '<textarea></textarea>' }))
			});
			flushSync();
			shellStore.setFocusedPane('web');
			flushSync();
			const focusRequest = shellStore.composerFocusRequest.id;

			if (kind === 'file') shellStore.closeFileTabForActive('a.md');
			else shellStore.closeUrlTabForActive(shellStore.workspacePanelUrlTabs[0]);
			flushSync();
			expect(shellStore.focusedPane).toBe('web');
			expect(container.querySelector('main')).not.toHaveClass('pane-focus-active');

			shellStore.closeWorkspacePanel();
			flushSync();
			expect(shellStore.focusedPane).toBe('web');
			expect(container.querySelector('main')).not.toHaveClass('pane-focus-active');
			expect(shellStore.composerFocusRequest.id).toBe(focusRequest);
		}
	);

	it('allows focus outside the workspace without a document-wide focus trap', async () => {
		const { container, getByRole } = render(AppShell, {
			children: createRawSnippet(() => ({ render: () => '<textarea></textarea>' }))
		});
		flushSync();
		await shellStore.openFilePreviewForActive('first.md');
		await waitFor(() => {
			expect(container.querySelector('.workspace-panel-inner')).toHaveClass(
				'pane-focus-active'
			);
		});

		const input = getByRole('textbox');
		input.focus();
		expect(input).toHaveFocus();
	});

	it('still transfers ownership for explicit chat actions and actual session changes', async () => {
		const { container, getByRole } = render(AppShell, {
			children: createRawSnippet(() => ({ render: () => '<textarea></textarea>' }))
		});
		flushSync();
		await shellStore.openFilePreviewForActive('first.md');
		flushSync();
		shellStore.requestComposerFocus();
		flushSync();
		expect(container.querySelector('main')).toHaveClass('pane-focus-active');

		shellStore.setFocusedPane('web');
		await fireEvent.mouseDown(getByRole('textbox'));
		expect(shellStore.focusedPane).toBe('chat');

		shellStore.setFocusedPane('web');
		sessionStore.selectSession({ ...session, title: 'Renamed' });
		flushSync();
		expect(shellStore.focusedPane).toBe('web');

		sessionStore.selectSession({ ...session, id: 'other-session' });
		flushSync();
		expect(shellStore.focusedPane).toBe('chat');
	});
});
