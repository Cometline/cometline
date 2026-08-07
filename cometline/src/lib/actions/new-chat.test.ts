import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
	goto: vi.fn().mockResolvedValue(undefined),
	detachActiveSession: vi.fn(),
	takePendingMessage: vi.fn(),
	createNewSession: vi.fn(),
	requestComposerFocus: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/stores/chat.svelte', () => ({
	chatStore: {
		sessionID: null,
		detachActiveSession: mocks.detachActiveSession,
		send: vi.fn()
	}
}));
vi.mock('$lib/stores/session.svelte', () => ({
	sessionStore: { current: null, takePendingMessage: mocks.takePendingMessage }
}));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: { requestComposerFocus: mocks.requestComposerFocus }
}));
vi.mock('$lib/actions/create-new-session', () => ({ createNewSession: mocks.createNewSession }));

import { startNewChat } from './new-chat';

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
		expect(mocks.goto.mock.invocationCallOrder[0]).toBeLessThan(
			mocks.requestComposerFocus.mock.invocationCallOrder[0]
		);
	});
});
