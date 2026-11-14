import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
	goto: vi.fn().mockResolvedValue(undefined),
	detachActiveSession: vi.fn(),
	takePendingMessage: vi.fn(),
	createNewSession: vi.fn(),
	requestComposerFocus: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('#lib/stores/chat.svelte.js', () => ({
	chatStore: {
		sessionID: null,
		detachActiveSession: mocks.detachActiveSession,
		send: vi.fn()
	}
}));
vi.mock('#lib/stores/session.svelte.js', () => ({
	sessionStore: { current: null, takePendingMessage: mocks.takePendingMessage }
}));
vi.mock('#lib/stores/shell.svelte.js', () => ({
	shellStore: { requestComposerFocus: mocks.requestComposerFocus }
}));
vi.mock('#lib/stores/app-toasts.svelte.js', () => ({
	appToastStore: { error: vi.fn() }
}));
vi.mock('#lib/actions/create-new-session.js', () => ({ createNewSession: mocks.createNewSession }));

import { appToastStore } from '#lib/stores/app-toasts.svelte.js';
import { reportNewChatStartError, startNewChat } from './new-chat';

describe('startNewChat', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mocks.createNewSession.mockResolvedValue({ id: 'new-session' });
	});

	it('targets the new session composer after navigating', async () => {
		await startNewChat();

		expect(mocks.detachActiveSession).toHaveBeenCalledOnce();
		expect(mocks.requestComposerFocus).toHaveBeenCalledWith('new-session');
		expect(mocks.goto).toHaveBeenCalledWith('/session/new-session');
		expect(mocks.createNewSession.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.detachActiveSession.mock.invocationCallOrder[0]
		);
	});

	it('creates the session in the requested workspace', async () => {
		await startNewChat('/group/workspace');

		expect(mocks.createNewSession).toHaveBeenCalledWith('/group/workspace');
	});

	it('surfaces create-session failures via toast', () => {
		const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
		reportNewChatStartError(new Error('invalid workspace'));

		expect(consoleError).toHaveBeenCalled();
		expect(appToastStore.error).toHaveBeenCalledWith(
			'Could not start new chat',
			'invalid workspace'
		);
		consoleError.mockRestore();
	});
});
