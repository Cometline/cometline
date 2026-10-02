import { createSession } from '#lib/client/cometmind.js';
import { modelStore } from '#lib/stores/model.svelte.js';
import { sessionStore } from '#lib/stores/session.svelte.js';
import { settingsStore } from '#lib/stores/settings.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { sessionVisitHistory } from '#lib/stores/session-visit-history.svelte.js';
import type { Session } from '#lib/types.js';

/** Create and activate a new persisted session using the configured default model. */
export async function createNewSession(workspacePath?: string): Promise<Session> {
	if (modelStore.options.length === 0) {
		await settingsStore.load();
	}
	modelStore.selectDefault();
	const model = modelStore.selected;
	if (!model) {
		throw new Error('Select a default model in Settings before starting a new chat.');
	}

	const session = await createSession({
		workspace_path: workspacePath || shellStore.defaultWorkspacePath,
		provider_id: model.providerId,
		model_id: model.modelId
	});
	sessionStore.appendSession(session);
	shellStore.commitActiveWorkspace(session.workspace_path);
	sessionVisitHistory.recordVisit(session.id);
	return session;
}
