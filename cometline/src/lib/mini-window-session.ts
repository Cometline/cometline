import { goto } from '$app/navigation';
import { createSession, listAllSessions } from '$lib/client/cometmind';
import { createNewSession } from '$lib/actions/create-new-session';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { sessionVisitHistory } from '$lib/stores/session-visit-history.svelte';
import { isDiscordSession } from '$lib/sessions/group-by-workspace';
import type { Session } from '$lib/types';

async function resolveSelectedModel() {
	if (modelStore.options.length === 0) {
		await settingsStore.load();
	}
	if (!modelStore.selected) {
		modelStore.selectDefault();
	}
	const selected = modelStore.selected;
	if (!selected) {
		throw new Error('Select a model in Settings before opening the mini window.');
	}
	return selected;
}

function isMiniWindowSessionExpired(state: MiniWindowState) {
	if (!state.sessionId) return true;
	if (state.lastActiveAt <= 0) return false;
	return Date.now() - state.lastActiveAt >= state.inactivityTimeoutMinutes * 60_000;
}

export async function ensureMiniWindowSession(preferredSessionId = '') {
	const state =
		(await window.electronAPI?.getMiniWindowState?.()) ??
		({
			sessionId: '',
			lastActiveAt: 0,
			inactivityTimeoutMinutes: 30
		} satisfies MiniWindowState);
	const sessionId = preferredSessionId || state.sessionId;
	const shouldReuseSession = preferredSessionId || !isMiniWindowSessionExpired(state);
	const workspacePath = (await window.electronAPI?.getWorkspacePath?.()) ?? '/';

	if (sessionId && shouldReuseSession) {
		// Mini sessions may be pinned or belong to another workspace. The route ID
		// is authoritative, so do not filter it through Electron's default workspace.
		const sessions = await listAllSessions();
		const session = sessions.sessions.find((item) => item.id === sessionId);
		if (session) {
			sessionStore.upsertSession(session);
			if (session.id !== state.sessionId) {
				await window.electronAPI?.saveMiniWindowState?.({ sessionId: session.id });
			}
			return session.id;
		}
	}

	const selected = await resolveSelectedModel();
	const session = await createSession({
		workspace_path: workspacePath,
		provider_id: selected.providerId,
		model_id: selected.modelId
	});
	await window.electronAPI?.saveMiniWindowState?.({ sessionId: session.id });
	sessionStore.appendSession(session);
	return session.id;
}

/** Select a session without leaving the mini-window route namespace. */
export async function navigateMiniToSession(session: Session) {
	sessionStore.selectSession(session);
	modelStore.selectFromSession(session);
	if (session.workspace_path && session.workspace_path !== shellStore.workspacePath) {
		shellStore.setActiveWorkspacePath(session.workspace_path);
	}
	if (session.workspace_path) {
		shellStore.setSidebarOrderWorkspacePath(session.workspace_path);
	}
	shellStore.setSidebarOrderDiscordActive(isDiscordSession(session));
	sessionVisitHistory.recordVisit(session.id);
	await window.electronAPI?.saveMiniWindowState?.({ sessionId: session.id });
	shellStore.requestComposerFocus(session.id);
	await goto(`/mini/session/${session.id}`);
}

/** Create and open a persisted mini-window session using the configured default model. */
export async function createMiniWindowSession() {
	const session = await createNewSession();
	await window.electronAPI?.saveMiniWindowState?.({ sessionId: session.id });
	shellStore.requestComposerFocus(session.id);
	await goto(`/mini/session/${session.id}`);
	return session;
}
