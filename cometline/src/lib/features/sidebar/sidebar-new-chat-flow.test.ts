// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
	goto: vi.fn().mockResolvedValue(undefined),
	detachActiveSession: vi.fn(),
	createNewSession: vi.fn(),
	requestComposerFocus: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('#lib/stores/chat.svelte.js', () => ({
	chatStore: { sessionID: null, detachActiveSession: mocks.detachActiveSession, send: vi.fn() }
}));
vi.mock('#lib/stores/session.svelte.js', () => ({
	sessionStore: { current: null, takePendingMessage: vi.fn() }
}));
vi.mock('#lib/stores/shell.svelte.js', () => ({
	shellStore: {
		defaultWorkspacePath: '/default-workspace',
		requestComposerFocus: mocks.requestComposerFocus
	}
}));
vi.mock('#lib/stores/app-toasts.svelte.js', () => ({
	appToastStore: { error: vi.fn(), success: vi.fn(), warning: vi.fn(), push: vi.fn() }
}));
vi.mock('#lib/actions/create-new-session.js', () => ({ createNewSession: mocks.createNewSession }));

import SidebarSearch from '#lib/features/sidebar/components/SidebarSearch.svelte';
import { startNewChat } from '#lib/actions/new-chat.js';

describe('sidebar new chat flow', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.createNewSession.mockResolvedValue({
			id: 'new-session-id',
			workspace_path: '/default-workspace'
		});
	});

	afterEach(() => {
		cleanup();
	});

	it('uses the default workspace path and navigates to the new session', async () => {
		const onNewChat = () => void startNewChat();
		const { getByRole } = render(SidebarSearch, { onNewChat });
		await fireEvent.click(getByRole('button', { name: 'New chat' }));

		await vi.waitFor(() => expect(mocks.createNewSession).toHaveBeenCalledOnce());
		expect(mocks.createNewSession).toHaveBeenCalledWith(undefined);
		await vi.waitFor(() => expect(mocks.goto).toHaveBeenCalledWith('/session/new-session-id'));
	});
});
