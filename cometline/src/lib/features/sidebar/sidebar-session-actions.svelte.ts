import type { Session } from '$lib/types';
import { sessionStore } from '$lib/stores/session.svelte';
import { deleteSession, updateSession } from '$lib/client/cometmind';
import {
	activateAfterSessionDeleted,
	sessionsSnapshot
} from '$lib/actions/activate-after-session-deleted';
import { shellStore } from '$lib/stores/shell.svelte';
import { terminalStore } from '$lib/stores/terminal.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';

export type SidebarContextMenu = { session: Session; x: number; y: number };

export function createSidebarSessionActions() {
	let deletingID = $state<string | null>(null);
	let pinningID = $state<string | null>(null);
	let contextMenu = $state<SidebarContextMenu | null>(null);
	let pendingDelete = $state<Session | null>(null);
	let terminalDeleteSession = $state<Session | null>(null);
	let pendingRename = $state<Session | null>(null);
	let renameTitle = $state('');

	async function removeSession(session: Session) {
		if (terminalStore.isRunning(session.id)) {
			terminalDeleteSession = session;
			return;
		}
		if (terminalStore.hasTerminal(session.id)) await terminalStore.remove(session.id);
		if (settingsStore.settings.app.confirmBeforeDeletingChats) {
			pendingDelete = session;
			return;
		}
		await deleteSelectedSession(session);
	}

	async function confirmTerminalDelete() {
		const session = terminalDeleteSession;
		if (!session) return;
		terminalDeleteSession = null;
		await terminalStore.remove(session.id);
		await deleteSelectedSession(session);
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		const session = pendingDelete;
		pendingDelete = null;
		await deleteSelectedSession(session);
	}

	async function alwaysDeleteWithoutConfirm() {
		// Persist preference in the background — don't block delete on settings IPC.
		void settingsStore.saveConfirmBeforeDeletingChats(false).catch(() => {});
		await confirmDelete();
	}

	async function deleteSelectedSession(session: Session) {
		deletingID = session.id;
		try {
			const wasCurrent = sessionStore.current?.id === session.id;
			const before = sessionsSnapshot();
			await deleteSession(session.id);
			shellStore.clearWorkspacePanelForSession(session.id);
			sessionStore.removeSession(session.id);
			if (wasCurrent) {
				await activateAfterSessionDeleted(session.id, before);
			}
		} finally {
			deletingID = null;
		}
	}

	async function togglePinSession(session: Session) {
		pinningID = session.id;
		try {
			const updated = await updateSession(session.id, { pinned: !session.pinned });
			sessionStore.updateSession(updated);
		} finally {
			pinningID = null;
		}
	}

	function openSessionContextMenu(session: Session, event: MouseEvent) {
		contextMenu = { session, x: event.clientX, y: event.clientY };
	}

	function closeSessionContextMenu() {
		contextMenu = null;
	}

	function startRenameSession(session: Session) {
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

	return {
		get deletingID() {
			return deletingID;
		},
		get pinningID() {
			return pinningID;
		},
		get contextMenu() {
			return contextMenu;
		},
		get pendingDelete() {
			return pendingDelete;
		},
		set pendingDelete(value: Session | null) {
			pendingDelete = value;
		},
		get terminalDeleteSession() {
			return terminalDeleteSession;
		},
		set terminalDeleteSession(value: Session | null) {
			terminalDeleteSession = value;
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
		removeSession,
		confirmTerminalDelete,
		confirmDelete,
		alwaysDeleteWithoutConfirm,
		togglePinSession,
		openSessionContextMenu,
		closeSessionContextMenu,
		startRenameSession,
		cancelRename,
		confirmRename
	};
}
