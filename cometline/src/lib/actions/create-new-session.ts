import { createSession } from '$lib/client/cometmind';
import { modelStore } from '$lib/stores/model.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { sessionVisitHistory } from '$lib/stores/session-visit-history.svelte';
import type { Session } from '$lib/types';

/** Create and activate a new persisted session using the configured default model. */
export async function createNewSession(): Promise<Session> {
	if (modelStore.options.length === 0) {
		await settingsStore.load();
	}
	modelStore.selectDefault();
	const model = modelStore.selected;
	if (!model) {
		throw new Error('Select a default model in Settings before starting a new chat.');
	}

	const session = await createSession({
		workspace_path: shellStore.defaultWorkspacePath,
		provider_id: model.providerId,
		model_id: model.modelId
	});
	sessionStore.appendSession(session);
	shellStore.commitActiveWorkspace(session.workspace_path);
	sessionVisitHistory.recordVisit(session.id);
	return session;
}
