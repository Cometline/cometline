import { createSession, listSessions } from '$lib/client/cometmind';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';

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
		({ sessionId: '', lastActiveAt: 0, inactivityTimeoutMinutes: 30 } satisfies MiniWindowState);
	const sessionId = preferredSessionId || state.sessionId;
	const shouldReuseSession = preferredSessionId || !isMiniWindowSessionExpired(state);
	const workspacePath = (await window.electronAPI?.getWorkspacePath?.()) ?? '/';

	if (sessionId && shouldReuseSession) {
		const sessions = await listSessions(workspacePath);
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
