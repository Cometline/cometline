// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import { tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const { emptyComponent } = vi.hoisted(() => ({ emptyComponent: () => ({}) }));
vi.mock('./FileTreeBrowser.svelte', () => ({ default: emptyComponent }));
vi.mock('./WorkspaceFileSurface.svelte', () => ({ default: emptyComponent }));
vi.mock('./GitChangesBrowser.svelte', () => ({ default: emptyComponent }));
vi.mock('./GitDiffView.svelte', () => ({ default: emptyComponent }));
vi.mock('./TerminalPanel.svelte', () => ({ default: emptyComponent }));
vi.mock('./ConfirmActionModal.svelte', () => ({ default: emptyComponent }));

import WorkspacePanel from './WorkspacePanel.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { deliverWindowSyncFromPeer } from '$lib/window-sync';

const session: Session = {
	id: 'web-lifecycle',
	workspace_id: 'repo',
	workspace_path: '/repo',
	title: 'Web tabs',
	model_id: 'model',
	provider_id: 'provider',
	status: 'active',
	origin: 'user',
	agent_mode: 'auto',
	pinned: false,
	running: false,
	created_at: 0,
	updated_at: 0,
	token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 }
};

// Real Svelte components/DOM; only Electron's native guest methods are simulated.
function attachGuest(container: HTMLElement, url: string) {
	const el = Array.from(
		container.querySelectorAll<HTMLElement & { src: string }>('webview')
	).find((el) => el.src === url)!;
	expect(el).toBeDefined();
	let requestedUrl = el.src;
	const srcWrites = vi.fn();
	Object.defineProperty(el, 'src', {
		configurable: true,
		get: () => requestedUrl,
		set: (value: string) => {
			requestedUrl = value;
			srcWrites(value);
		}
	});
	const native = {
		stop: vi.fn(),
		reload: vi.fn(),
		goBack: vi.fn(),
		goForward: vi.fn(),
		canGoBack: vi.fn(() => false),
		canGoForward: vi.fn(() => false),
		getURL: vi.fn(() => url),
		getTitle: vi.fn(() => ''),
		executeJavaScript: vi.fn(async () => ({
			url,
			title: 'Captured page',
			content: 'Page body'
		}))
	};
	Object.assign(el, native);
	return {
		el,
		...native,
		srcWrites,
		async navigate(nextUrl: string, title: string) {
			native.getURL.mockReturnValue(nextUrl);
			native.getTitle.mockReturnValue(title);
			el.dispatchEvent(new Event('did-navigate'));
			await tick();
		}
	};
}

describe('WorkspacePanel web tab lifetimes', () => {
	beforeEach(() => {
		shellStore.clearWorkspacePanelForSession(session.id);
		shellStore.clearWorkspacePanelForSession('other-session');
		sessionStore.selectSession(session);
		shellStore.commitActiveWorkspace('/repo');
	});

	afterEach(() => {
		cleanup();
		shellStore.clearWorkspacePanelForSession(session.id);
		shellStore.clearWorkspacePanelForSession('other-session');
		sessionStore.selectSession(null);
	});

	it('retains each guest across tab switches and only destroys the closed tab', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://example.com');
		const [firstId] = shellStore.workspacePanelUrlTabs;
		const { container, getByRole } = render(WorkspacePanel);
		await tick();
		const first = attachGuest(container, 'https://example.com');
		await first.navigate('https://example.com', 'Example');

		shellStore.openWorkspacePanelUrlForActive('https://www.youtube.com/watch?v=1');
		const [, youtubeId] = shellStore.workspacePanelUrlTabs;
		await tick();
		const youtube = attachGuest(container, 'https://www.youtube.com/watch?v=1');
		await youtube.navigate('https://www.youtube.com/watch?v=1', 'YouTube');
		expect(first.el.isConnected).toBe(true);
		expect(first.stop).not.toHaveBeenCalled();

		await fireEvent.click(getByRole('tab', { name: 'Example' }));
		expect(youtube.el.isConnected).toBe(true);
		expect(youtube.el.parentElement).toHaveAttribute('aria-hidden', 'true');
		expect(youtube.stop).not.toHaveBeenCalled();
		await fireEvent.click(getByRole('tab', { name: 'YouTube' }));
		expect(youtube.el.parentElement).toHaveAttribute('aria-hidden', 'false');
		expect(youtube.srcWrites).not.toHaveBeenCalled();

		// Closing an earlier tab changes the array index, not the surviving guest identity.
		shellStore.closeUrlTabForActive(firstId);
		await tick();
		expect(first.el.isConnected).toBe(false);
		expect(first.stop).toHaveBeenCalledOnce();
		expect(youtube.el.isConnected).toBe(true);
		expect(youtube.srcWrites).not.toHaveBeenCalled();
		expect(youtube.stop).not.toHaveBeenCalled();

		shellStore.closeUrlTabForActive(youtubeId);
		await tick();
		expect(youtube.el.isConnected).toBe(false);
		expect(youtube.stop).toHaveBeenCalledOnce();
		expect(container.querySelectorAll('webview')).toHaveLength(0);
	});

	it('isolates background navigation, loading, focus, and active toolbar commands', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://background.example');
		const [backgroundId] = shellStore.workspacePanelUrlTabs;
		const { container, getByRole } = render(WorkspacePanel);
		await tick();
		const background = attachGuest(container, 'https://background.example');
		await background.navigate('https://background.example', 'Background');
		shellStore.openWorkspacePanelUrlForActive('https://active.example');
		await tick();
		const active = attachGuest(container, 'https://active.example');
		active.canGoBack.mockReturnValue(true);
		await active.navigate('https://active.example', 'Active');
		shellStore.setFocusedPane('chat');

		background.canGoForward.mockReturnValue(true);
		await background.navigate('https://background.example/next', 'Background next');
		background.el.dispatchEvent(new Event('did-start-loading'));
		background.el.dispatchEvent(new Event('focus'));
		await tick();
		expect(shellStore.workspacePanelUrl).toBe('https://active.example');
		expect(shellStore.workspacePanelUrlTabMeta[backgroundId]).toEqual({
			url: 'https://background.example/next',
			title: 'Background next'
		});
		expect(shellStore.focusedPane).toBe('chat');
		expect(shellStore.pendingWebContexts).toEqual([
			expect.objectContaining({ source: 'https://active.example', title: 'Active' })
		]);
		expect(getByRole('button', { name: 'Forward' })).toBeDisabled();
		expect(getByRole('button', { name: 'Reload page' }).querySelector('.spin')).toBeNull();
		await fireEvent.click(getByRole('button', { name: 'Back' }));
		await fireEvent.click(getByRole('button', { name: 'Reload page' }));
		expect(active.goBack).toHaveBeenCalledOnce();
		expect(active.reload).toHaveBeenCalledOnce();
		expect(background.goBack).not.toHaveBeenCalled();
		expect(background.reload).not.toHaveBeenCalled();

		await fireEvent.click(getByRole('tab', { name: 'Background next' }));
		expect(getByRole('button', { name: 'Forward' })).toBeEnabled();
		expect(getByRole('button', { name: 'Reload page' }).querySelector('.spin')).not.toBeNull();
		expect(background.srcWrites).not.toHaveBeenCalled();
		expect(background.stop).not.toHaveBeenCalled();
	});

	it('retains guests while browsing files, hiding the pane, or switching sessions', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://www.youtube.com');
		const [youtubeId] = shellStore.workspacePanelUrlTabs;
		const { container } = render(WorkspacePanel);
		await tick();
		const youtube = attachGuest(container, 'https://www.youtube.com');
		await youtube.navigate('https://www.youtube.com', 'YouTube');

		await shellStore.openFilePreviewForActive('README.md');
		await tick();
		await youtube.navigate('https://www.youtube.com/watch?v=2', 'Next video');
		expect(shellStore.contentSurface).toBe('workspace');
		expect(shellStore.workspacePanelFilePath).toBe('README.md');
		expect(youtube.el.isConnected).toBe(true);

		shellStore.toggleWorkspacePanel();
		await tick();
		await youtube.navigate('https://www.youtube.com/watch?v=3', 'Third video');
		expect(shellStore.workspacePanelOpen).toBe(false);
		expect(youtube.el.isConnected).toBe(true);

		sessionStore.selectSession({ ...session, id: 'other-session' });
		await tick();
		expect(youtube.el.isConnected).toBe(true);
		shellStore.openWorkspacePanelUrlForActive('https://other.example');
		await tick();
		const other = attachGuest(container, 'https://other.example');
		await other.navigate('https://other.example', 'Other session');
		await youtube.navigate('https://www.youtube.com/watch?v=4', 'Fourth video');
		expect(shellStore.workspacePanelUrl).toBe('https://other.example');
		expect(shellStore.pendingWebContexts).toEqual([
			expect.objectContaining({ source: 'https://other.example', title: 'Other session' })
		]);
		expect(youtube.el.isConnected).toBe(true);

		sessionStore.selectSession(session);
		shellStore.activateUrlTabForActive(youtubeId);
		await tick();
		expect(shellStore.workspacePanelUrl).toBe('https://www.youtube.com/watch?v=4');
		expect(youtube.el.parentElement).toHaveAttribute('aria-hidden', 'false');
		expect(youtube.srcWrites).not.toHaveBeenCalled();
		expect(youtube.stop).not.toHaveBeenCalled();

		shellStore.clearWorkspacePanelForSession(session.id);
		await tick();
		expect(youtube.stop).toHaveBeenCalledOnce();
		expect(youtube.el.isConnected).toBe(false);
		youtube.el.dispatchEvent(new Event('did-navigate'));
		await tick();
		expect(shellStore.workspaceWebTabs).toEqual([
			expect.objectContaining({ sessionId: 'other-session', url: 'https://other.example' })
		]);
		expect(other.el.isConnected).toBe(true);
		expect(other.stop).not.toHaveBeenCalled();
	});

	it('does not undo address navigation when loading events still report the old URL', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://old.example');
		const { container } = render(WorkspacePanel);
		await tick();
		const guest = attachGuest(container, 'https://old.example');
		await guest.navigate('https://old.example', 'Old page');

		shellStore.navigateWorkspacePanel('https://new.example');
		await tick();
		guest.el.dispatchEvent(new Event('did-start-loading'));
		await tick();
		expect(shellStore.workspacePanelUrl).toBe('https://new.example');
		expect(guest.srcWrites.mock.calls).toEqual([['https://new.example']]);
		await guest.navigate('https://new.example', 'New page');
		expect(guest.srcWrites).toHaveBeenCalledOnce();
	});

	it('retains a loaded guest while opening a blank tab and resolves its pending context', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://www.youtube.com');
		const { container } = render(WorkspacePanel);
		await tick();
		const youtube = attachGuest(container, 'https://www.youtube.com');
		await youtube.navigate('https://www.youtube.com', 'YouTube');
		shellStore.openWebSearchPanel();
		await tick();
		expect(container.querySelectorAll('webview')).toHaveLength(2);
		expect(youtube.el.isConnected).toBe(true);
		expect(youtube.stop).not.toHaveBeenCalled();
		expect(youtube.srcWrites).not.toHaveBeenCalled();

		const contexts = await shellStore.resolvePendingWebContextsForActive();
		expect(contexts).toEqual([
			expect.objectContaining({ source: 'https://www.youtube.com', content: 'Page body' })
		]);
		expect(youtube.executeJavaScript).toHaveBeenCalledOnce();
	});

	it('discards an in-flight capture when its guest is closed', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://example.com');
		const [tabId] = shellStore.workspacePanelUrlTabs;
		const { container } = render(WorkspacePanel);
		await tick();
		const guest = attachGuest(container, 'https://example.com');
		let rejectCapture!: (error: Error) => void;
		guest.executeJavaScript.mockImplementation(
			() =>
				new Promise((_resolve, reject) => {
					rejectCapture = reject;
				})
		);
		const pending = shellStore.resolvePendingWebContextsForActive();
		shellStore.closeUrlTabForActive(tabId);
		await tick();
		rejectCapture(new Error('Guest destroyed'));
		await expect(pending).resolves.toEqual([]);
	});

	it('does not attach an in-flight capture to another session', async () => {
		shellStore.openWorkspacePanelUrlForActive('https://example.com');
		const { container, getByRole } = render(WorkspacePanel);
		await tick();
		const guest = attachGuest(container, 'https://example.com');
		let resolveCapture!: (value: { url: string; title: string; content: string }) => void;
		guest.executeJavaScript.mockImplementation(
			() =>
				new Promise((resolve) => {
					resolveCapture = resolve;
				})
		);
		await fireEvent.click(getByRole('button', { name: 'Add page to chat context' }));
		sessionStore.selectSession({ ...session, id: 'other-session' });
		resolveCapture({ url: 'https://example.com', title: 'Example', content: 'Body' });
		await tick();
		expect(shellStore.pendingWebContexts).toEqual([]);
	});

	it('resolves context from the selected tab when two guests have the same URL', async () => {
		const sharedUrl = 'https://example.com/dashboard';
		shellStore.openWorkspacePanelUrlForActive(sharedUrl);
		const { container } = render(WorkspacePanel);
		await tick();
		const first = attachGuest(container, sharedUrl);
		await first.navigate(sharedUrl, 'First dashboard');
		shellStore.openWorkspacePanelUrlForActive('https://second.example');
		await tick();
		const second = attachGuest(container, 'https://second.example');
		await second.navigate(sharedUrl, 'Second dashboard');
		second.executeJavaScript.mockResolvedValue({
			url: sharedUrl,
			title: 'Second dashboard',
			content: 'Selected tab content'
		});

		const contexts = await shellStore.resolvePendingWebContextsForActive();
		expect(contexts).toEqual([expect.objectContaining({ content: 'Selected tab content' })]);
		expect(first.executeJavaScript).not.toHaveBeenCalled();
	});

	it.each(['local', 'peer'] as const)(
		'destroys guests when a session is removed by %s',
		async (source) => {
			shellStore.openWorkspacePanelUrlForActive('https://example.com');
			const { container } = render(WorkspacePanel);
			await tick();
			const guest = attachGuest(container, 'https://example.com');
			if (source === 'peer')
				deliverWindowSyncFromPeer({ type: 'session-remove', sessionId: session.id });
			else sessionStore.removeSession(session.id);
			await tick();
			expect(guest.el.isConnected).toBe(false);
			expect(guest.stop).toHaveBeenCalledOnce();
			expect(shellStore.workspaceWebTabs).toEqual([]);
		}
	);
});
