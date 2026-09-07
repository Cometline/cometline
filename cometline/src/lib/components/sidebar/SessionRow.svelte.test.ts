// @vitest-environment jsdom
import { cleanup, fireEvent, render, within } from '@testing-library/svelte';
import { tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const { navigateToSession } = vi.hoisted(() => ({ navigateToSession: vi.fn() }));
vi.mock('$lib/actions/navigate-to-session', () => ({ navigateToSession }));
vi.mock('$lib/stores/chat.svelte', () => ({
	chatStore: { isStreamingFor: () => false, hasRunError: () => false }
}));

import SessionRow from './SessionRow.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { webTabActivity, type WebTabActivity } from '$lib/workspace/web-tab-activity.svelte';

const session: Session = {
	id: 'audio-session',
	workspace_id: 'repo',
	workspace_path: '/repo',
	title: 'Audio session',
	model_id: 'model',
	provider_id: 'provider',
	status: 'active',
	origin: 'user',
	agent_mode: 'auto',
	pinned: false,
	running: true,
	created_at: 0,
	updated_at: 0,
	token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 }
};

function addAudioTab(title: string) {
	const url = `https://${title.toLowerCase()}.example`;
	shellStore.openWorkspacePanelUrl(url, session.id);
	const tab = shellStore.workspaceWebTabs.find((tab) => tab.url === url)!;
	const pageState = $state({
		url,
		title,
		canGoBack: false,
		canGoForward: false,
		loading: false,
		showLoading: false,
		loadError: null,
		ready: true,
		mediaPlaying: true,
		audible: true,
		muted: false,
		capturing: false
	});
	const surface: WebTabActivity['surface'] = {
		pageState,
		navigateBack: vi.fn(() => false),
		navigateForward: vi.fn(() => false),
		reload: vi.fn(),
		focus: vi.fn(),
		captureContext: vi.fn(async () => null),
		toggleAudioMuted: vi.fn(() => {
			pageState.muted = !pageState.muted;
			pageState.audible = !pageState.muted;
		})
	};
	const entry = { sessionId: session.id, tabId: tab.id, surface };
	webTabActivity.set(tab.key, entry);
	return { ...entry, key: tab.key };
}

describe('SessionRow audio indicator', () => {
	beforeEach(() => {
		webTabActivity.clear();
		shellStore.clearWorkspacePanelForSession(session.id);
		sessionStore.selectSession({ ...session, id: 'other-session' });
		navigateToSession.mockReset();
		navigateToSession.mockImplementation(async (target: Session) => {
			sessionStore.selectSession(target);
			shellStore.requestComposerFocus(target.id);
		});
	});
	afterEach(() => {
		cleanup();
		webTabActivity.clear();
		shellStore.clearWorkspacePanelForSession(session.id);
		sessionStore.selectSession(null);
		vi.restoreAllMocks();
	});

	it('replaces the existing dot with audio instead of showing both indicators', async () => {
		const tab = addAudioTab('Music');
		const onSelect = vi.fn();
		const { container, getByRole } = render(SessionRow, {
			session,
			onSelect,
			onDelete: vi.fn(),
			onPin: vi.fn(),
			onContextMenu: vi.fn()
		});
		const badge = getByRole('button', { name: 'Audio tabs for Audio session' });
		expect(badge.closest('.session-audio')).not.toBeNull();
		expect(badge.closest('.session-actions')).toBeNull();
		expect(badge.closest('.session-row')).toBeNull();
		expect(container.querySelector('.session-row-wrap')).toHaveClass('has-audio');
		expect(container.querySelector('.session-streaming')).toBeNull();
		await fireEvent.click(badge);
		expect(navigateToSession).toHaveBeenCalledWith(session);
		expect(onSelect).not.toHaveBeenCalled();
		expect(shellStore.workspacePanelUrlTabId).toBe(tab.tabId);
		expect(shellStore.focusedPane).toBe('web');
	});

	it('opens an audio chooser with keyboard focus and mutes without navigating', async () => {
		const music = addAudioTab('Music');
		addAudioTab('Video');
		shellStore.setFocusedPane('chat');
		const { getByRole, queryByRole } = render(SessionRow, {
			session,
			onSelect: vi.fn(),
			onDelete: vi.fn(),
			onPin: vi.fn(),
			onContextMenu: vi.fn()
		});
		const badge = getByRole('button', { name: 'Audio tabs for Audio session' });
		await fireEvent.click(badge);
		const chooser = getByRole('dialog', { name: 'Audio tabs' });
		expect(within(chooser).getAllByRole('button')[0]).toHaveFocus();
		await fireEvent.click(within(chooser).getByRole('button', { name: 'Mute Music' }));
		expect(music.surface.toggleAudioMuted).toHaveBeenCalledOnce();
		expect(within(chooser).getByRole('button', { name: 'Unmute Music' })).toHaveAttribute(
			'aria-pressed',
			'true'
		);
		expect(navigateToSession).not.toHaveBeenCalled();
		expect(shellStore.focusedPane).toBe('chat');
		await fireEvent.keyDown(window, { key: 'Escape' });
		expect(queryByRole('dialog')).toBeNull();
		expect(badge).toHaveFocus();
	});

	it('keeps muted controls available and restores the dot after all audio tabs close', async () => {
		const music = addAudioTab('Music');
		const video = addAudioTab('Video');
		const { container, getByRole, queryByRole } = render(SessionRow, {
			session,
			onSelect: vi.fn(),
			onDelete: vi.fn(),
			onPin: vi.fn(),
			onContextMenu: vi.fn()
		});
		music.surface.toggleAudioMuted();
		video.surface.toggleAudioMuted();
		await tick();
		const badge = getByRole('button', { name: 'Audio tabs for Audio session' });
		expect(badge).toHaveClass('muted');
		await fireEvent.click(badge);
		expect(getByRole('dialog')).toBeTruthy();
		webTabActivity.clear();
		await tick();
		expect(queryByRole('dialog')).toBeNull();
		expect(queryByRole('button', { name: 'Audio tabs for Audio session' })).toBeNull();
		expect(container.querySelector('.session-row-wrap')).not.toHaveClass('has-audio');
		expect(container.querySelector('.session-streaming')).toHaveClass('active');
	});

	it('closes the chooser on an outside click without selecting the session', async () => {
		addAudioTab('Music');
		addAudioTab('Video');
		const onSelect = vi.fn();
		const { getByRole, queryByRole } = render(SessionRow, {
			session,
			onSelect,
			onDelete: vi.fn(),
			onPin: vi.fn(),
			onContextMenu: vi.fn()
		});
		await fireEvent.click(getByRole('button', { name: 'Audio tabs for Audio session' }));
		await fireEvent.pointerDown(document.body);
		expect(queryByRole('dialog')).toBeNull();
		expect(onSelect).not.toHaveBeenCalled();
		expect(navigateToSession).not.toHaveBeenCalled();
	});

	it('dismisses the narrow sidebar when revealing a playing tab', async () => {
		addAudioTab('Music');
		shellStore.openSidebar();
		const media = window.matchMedia('(max-width: 900px)');
		vi.spyOn(window, 'matchMedia').mockReturnValue({ ...media, matches: true });
		const { getByRole } = render(SessionRow, {
			session,
			onSelect: vi.fn(),
			onDelete: vi.fn(),
			onPin: vi.fn(),
			onContextMenu: vi.fn()
		});
		await fireEvent.click(getByRole('button', { name: 'Audio tabs for Audio session' }));
		expect(shellStore.sidebarOpen).toBe(false);
		expect(shellStore.workspacePanelUrl).toBe('https://music.example');
	});
});
