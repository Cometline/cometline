import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Session } from '$lib/types';

const mocks = vi.hoisted(() => ({
	createSession: vi.fn(),
	selectDefault: vi.fn(),
	appendSession: vi.fn(),
	commitActiveWorkspace: vi.fn(),
	recordVisit: vi.fn(),
	loadSettings: vi.fn(),
	defaultModel: {
		id: 'provider:model',
		label: 'Model',
		providerId: 'provider',
		providerName: 'Provider',
		providerMethod: 'openai' as const,
		modelId: 'model'
	}
}));

vi.mock('$lib/client/cometmind', () => ({ createSession: mocks.createSession }));
vi.mock('$lib/stores/model.svelte', () => ({
	modelStore: {
		options: [mocks.defaultModel],
		selected: mocks.defaultModel,
		selectDefault: mocks.selectDefault
	}
}));
vi.mock('$lib/stores/session.svelte', () => ({
	sessionStore: { appendSession: mocks.appendSession }
}));
vi.mock('$lib/stores/settings.svelte', () => ({
	settingsStore: { load: mocks.loadSettings }
}));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		defaultWorkspacePath: '/default-workspace',
		commitActiveWorkspace: mocks.commitActiveWorkspace
	}
}));
vi.mock('$lib/stores/session-visit-history.svelte', () => ({
	sessionVisitHistory: { recordVisit: mocks.recordVisit }
}));

import { createNewSession } from './create-new-session';

const session: Session = {
	id: 'session-1',
	workspace_id: 'workspace-1',
	workspace_path: '/default-workspace',
	title: '',
	model_id: 'model',
	provider_id: 'provider',
	status: 'active',
	origin: 'user',
	token_usage: { input_tokens: 0, output_tokens: 0, cache_read: 0, cache_write: 0 },
	pinned: false,
	agent_mode: 'auto',
	running: false,
	created_at: 0,
	updated_at: 0
};

describe('createNewSession', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.createSession.mockResolvedValue(session);
	});

	it('creates and activates a persisted session with the configured default model', async () => {
		await expect(createNewSession()).resolves.toEqual(session);

		expect(mocks.selectDefault).toHaveBeenCalledOnce();
		expect(mocks.createSession).toHaveBeenCalledWith({
			workspace_path: '/default-workspace',
			provider_id: 'provider',
			model_id: 'model'
		});
		expect(mocks.appendSession).toHaveBeenCalledWith(session);
		expect(mocks.commitActiveWorkspace).toHaveBeenCalledWith('/default-workspace');
		expect(mocks.recordVisit).toHaveBeenCalledWith('session-1');
	});
});
