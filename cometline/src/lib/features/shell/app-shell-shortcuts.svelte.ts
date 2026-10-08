import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { navigateToSession } from '#lib/actions/navigate-to-session.js';
import { startNewChat } from '#lib/actions/new-chat.js';
import { navigateAdjacentSession } from '#lib/actions/navigate-adjacent-session.js';
import {
	navigateSessionHistory,
	navigateToRecentSession
} from '#lib/actions/navigate-session-history.js';
import { openSettings } from '#lib/actions/open-settings.js';
import { shouldUseWorkspacePanelHistory } from '#lib/features/shell/focus-nav.js';
import { isReloadShortcut, matchesShortcut, type ShortcutAction } from '#lib/keyboard-shortcuts.js';
import { inboxStore } from '#lib/stores/inbox.svelte.js';
import { sessionStore } from '#lib/stores/session.svelte.js';
import { settingsStore } from '#lib/stores/settings.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { getSession, updateSession } from '#lib/client/cometmind.js';
import type { Session } from '#lib/types.js';

type SidebarHandle = { focusSearch: () => void };
type WorkspacePanelHandle = { navigateBack: () => void; navigateForward: () => void };

const PRIMARY_SHORTCUTS: ShortcutAction[] = [
	'toggleSidebar',
	'toggleWorkspacePanel',
	'openWebSearch',
	'openGitPanel',
	'openWikiPanel',
	'openWorkspacePanel',
	'openFileSearch',
	'openTerminal',
	'navigateBack',
	'navigateForward',
	'openSettings',
	'newChat',
	'findInSession',
	'focusSearch',
	'openJobs',
	'openSkillDrafts',
	'openGallery',
	'openUsage',
	'openInbox',
	'recentSession'
];

export function createAppShellShortcuts(deps: {
	getSidebarRef: () => SidebarHandle | null;
	getWorkspacePanelRef: () => WorkspacePanelHandle | null;
	canFindInSession: () => boolean;
}) {
	let closeConfirmOpen = $state(false);
	let fileSearchOpen = $state(false);
	let reloadConfirmOpen = $state(false);
	let reloadConfirmArmed = $state(false);
	let pendingRename = $state<Session | null>(null);
	let renameTitle = $state('');

	function startRenameFromTitlebar() {
		const session = sessionStore.current;
		if (!session) return;
		renameTitle = session.title || '';
		pendingRename = session;
	}

	function cancelRename() {
		pendingRename = null;
	}

	async function confirmRename() {
		if (!pendingRename) return;
		const updated = await updateSession(pendingRename.id, { title: renameTitle.trim() });
		sessionStore.updateSession(updated);
		pendingRename = null;
	}

	function isCmdW(event: KeyboardEvent) {
		return (
			event.metaKey &&
			!event.ctrlKey &&
			!event.altKey &&
			!event.shiftKey &&
			event.key.toLowerCase() === 'w'
		);
	}

	function hideMainWindow() {
		closeConfirmOpen = false;
		window.electronAPI?.confirmCloseWindow?.();
	}

	function handleRequestCloseWindow() {
		if (closeConfirmOpen) {
			hideMainWindow();
			return;
		}
		if (inboxStore.drawerOpen) {
			inboxStore.closeDrawer();
			return;
		}
		if (shellStore.workspacePanelOpen) {
			shellStore.closeWorkspacePanel();
			return;
		}
		if (settingsStore.settings.app.confirmCloseOnCmdW === false) {
			hideMainWindow();
			return;
		}
		closeConfirmOpen = true;
	}

	function confirmReload() {
		reloadConfirmOpen = false;
		reloadConfirmArmed = false;
		window.location.reload();
	}

	function cancelReload() {
		reloadConfirmOpen = false;
		reloadConfirmArmed = false;
	}

	function handleRequestReload() {
		if (reloadConfirmOpen) {
			if (!reloadConfirmArmed) return;
			confirmReload();
			return;
		}
		reloadConfirmOpen = true;
		reloadConfirmArmed = false;
		queueMicrotask(() => {
			if (reloadConfirmOpen) reloadConfirmArmed = true;
		});
	}

	async function alwaysCloseWithoutConfirm() {
		void settingsStore.saveConfirmCloseOnCmdW(false).catch(() => {});
		hideMainWindow();
	}

	function runShortcutAction(action: ShortcutAction) {
		switch (action) {
			case 'toggleSidebar':
				shellStore.toggleSidebar();
				return;
			case 'toggleWorkspacePanel':
				shellStore.toggleWorkspacePanel();
				return;
			case 'openWebSearch':
				shellStore.openWebSearchPanel();
				return;
			case 'openGitPanel':
				shellStore.openGitChangesPanel();
				return;
			case 'openWikiPanel':
				shellStore.setWorkspacePanelBrowseSource('wiki');
				return;
			case 'openWorkspacePanel':
				shellStore.setWorkspacePanelBrowseSource('workspace');
				return;
			case 'openFileSearch':
				if (shellStore.settingsOpen) return;
				fileSearchOpen = true;
				return;
			case 'openTerminal':
				shellStore.requestTerminalFocus();
				return;
			case 'navigateBack':
				if (
					shouldUseWorkspacePanelHistory(
						shellStore.workspacePanelOpen,
						!!deps.getWorkspacePanelRef()
					)
				) {
					deps.getWorkspacePanelRef()?.navigateBack();
				} else {
					navigateSessionHistory('back');
				}
				return;
			case 'navigateForward':
				if (
					shouldUseWorkspacePanelHistory(
						shellStore.workspacePanelOpen,
						!!deps.getWorkspacePanelRef()
					)
				) {
					deps.getWorkspacePanelRef()?.navigateForward();
				} else {
					navigateSessionHistory('forward');
				}
				return;
			case 'openSettings':
				openSettings();
				return;
			case 'newChat':
				startNewChat();
				return;
			case 'findInSession':
				if (!deps.canFindInSession()) return;
				shellStore.requestSessionFind();
				return;
			case 'focusSearch':
				shellStore.openSidebar();
				deps.getSidebarRef()?.focusSearch();
				return;
			case 'previousSession':
				if (shellStore.settingsOpen) return;
				navigateAdjacentSession('prev');
				return;
			case 'nextSession':
				if (shellStore.settingsOpen) return;
				navigateAdjacentSession('next');
				return;
			case 'openJobs':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto(resolve('jobs'));
				return;
			case 'openSkillDrafts':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto(resolve('skills'));
				return;
			case 'openGallery':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto(resolve('gallery'));
				return;
			case 'openUsage':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto(resolve('usage'));
				return;
			case 'openInbox':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.toggleDrawer();
				return;
			case 'recentSession':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				navigateToRecentSession();
				return;
			default:
				return;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		const shortcuts = settingsStore.settings.shortcuts;
		if (closeConfirmOpen && event.key === 'Escape') {
			event.preventDefault();
			closeConfirmOpen = false;
			return;
		}
		if (reloadConfirmOpen && event.key === 'Escape') {
			event.preventDefault();
			cancelReload();
			return;
		}
		if (isCmdW(event)) {
			event.preventDefault();
			handleRequestCloseWindow();
			return;
		}
		if (
			isReloadShortcut({
				key: event.key,
				code: event.code,
				meta: event.metaKey,
				control: event.ctrlKey,
				alt: event.altKey,
				shift: event.shiftKey,
				isComposing: event.isComposing
			})
		) {
			event.preventDefault();
			handleRequestReload();
			return;
		}
		if (
			shellStore.focusedPane === 'terminal' &&
			shellStore.terminalPanelOpen &&
			!shellStore.settingsOpen &&
			!inboxStore.drawerOpen &&
			event.key === 'Escape'
		) {
			return;
		}
		if (matchesShortcut(event, shortcuts.closeSettings) && shellStore.settingsOpen) {
			event.preventDefault();
			shellStore.closeSettings();
			return;
		}
		if (matchesShortcut(event, shortcuts.cycleReasoningEffort)) return;

		for (const action of PRIMARY_SHORTCUTS) {
			if (!matchesShortcut(event, shortcuts[action])) continue;
			if (action === 'findInSession' && !deps.canFindInSession()) return;
			event.preventDefault();
			runShortcutAction(action);
			return;
		}
		if (shellStore.settingsOpen) return;
		for (const action of ['previousSession', 'nextSession'] as const) {
			if (!matchesShortcut(event, shortcuts[action])) continue;
			event.preventDefault();
			runShortcutAction(action);
			return;
		}
	}

	function install() {
		window.addEventListener('keydown', handleKeydown, true);
		const unsubscribeNavigate = window.electronAPI?.onNavigateSession?.((direction) => {
			if (shellStore.settingsOpen) return;
			navigateAdjacentSession(direction);
		});
		const unsubscribeCloseWorkspacePanel = window.electronAPI?.onCloseWorkspacePanel?.(() => {
			shellStore.closeWorkspacePanel();
		});
		const unsubscribeCloseInbox = window.electronAPI?.onCloseInbox?.(() => {
			inboxStore.closeDrawer();
		});
		const unsubscribeRequestCloseWindow = window.electronAPI?.onRequestCloseWindow?.(() => {
			handleRequestCloseWindow();
		});
		const unsubscribeRequestReload = window.electronAPI?.onRequestReload?.(() => {
			handleRequestReload();
		});
		const unsubscribeShortcutAction = window.electronAPI?.onShortcutAction?.((action) => {
			runShortcutAction(action);
		});
		const unsubscribeReplayIntro = window.electronAPI?.onReplayIntro?.(() => {
			shellStore.openIntro();
		});
		const unsubscribeOpenSession = window.electronAPI?.onOpenSession?.((sessionId) => {
			void getSession(sessionId)
				.then((session) => navigateToSession(session))
				.catch(() => undefined);
		});
		const unsubscribeRunSetupWizard = window.electronAPI?.onRunSetupWizard?.(() => {
			shellStore.openSetup();
		});
		return () => {
			window.removeEventListener('keydown', handleKeydown, true);
			unsubscribeNavigate?.();
			unsubscribeCloseWorkspacePanel?.();
			unsubscribeCloseInbox?.();
			unsubscribeRequestCloseWindow?.();
			unsubscribeRequestReload?.();
			unsubscribeShortcutAction?.();
			unsubscribeReplayIntro?.();
			unsubscribeRunSetupWizard?.();
			unsubscribeOpenSession?.();
		};
	}

	return {
		get closeConfirmOpen() {
			return closeConfirmOpen;
		},
		set closeConfirmOpen(value: boolean) {
			closeConfirmOpen = value;
		},
		get fileSearchOpen() {
			return fileSearchOpen;
		},
		set fileSearchOpen(value: boolean) {
			fileSearchOpen = value;
		},
		get reloadConfirmOpen() {
			return reloadConfirmOpen;
		},
		get pendingRename() {
			return pendingRename;
		},
		get renameTitle() {
			return renameTitle;
		},
		set renameTitle(value: string) {
			renameTitle = value;
		},
		startRenameFromTitlebar,
		cancelRename,
		confirmRename,
		hideMainWindow,
		confirmReload,
		cancelReload,
		alwaysCloseWithoutConfirm,
		install
	};
}
